Dưới đây là bản tổng hợp sau khi đã áp dụng các chỉnh sửa. Thứ tự trình bày: nguyên tắc nền, schema, mã giả từng service, rồi ma trận edge case có trỏ ngược về đoạn mã xử lý.

## 0. Nguyên tắc nền

Toàn bộ thiết kế dựa trên 5 quy tắc. Mỗi edge case ở mục 7 đều được giải quyết bởi một hoặc vài quy tắc này.

1. **Không transaction nào chứa network call.** Các lời gọi gRPC, MoMo và RabbitMQ đều nằm giữa các transaction ngắn.
2. **Mọi chuyển trạng thái đơn đều khóa dòng và kiểm trạng thái nguồn** (`SELECT ... FOR UPDATE` rồi `if status != expected`). Bên nào chốt trước thì thắng, bên đến sau thoát ra.
3. **Đổi trạng thái và ghi outbox nằm trong cùng một transaction.** Mỗi đơn phát tối đa một event cuối: `OrderConfirmed` hoặc `ReservationReleaseRequested`.
4. **Mọi consumer idempotent theo khóa nghiệp vụ** và chỉ ACK sau khi đã commit.
5. **Chiếm `Idempotency-Key` cùng transaction với việc tạo đơn**, đặt sau mọi bước chỉ đọc và trước mọi tác dụng phụ.

Máy trạng thái của đơn:

```
                 ┌─(COD, giữ kho OK)──────────────→ CONFIRMED
PENDING_STOCK ───┼─(MoMo, giữ kho + payUrl OK)──→ PENDING_PAYMENT ──(IPN/redirect OK)──→ CONFIRMED
                 └─(hết hàng / lỗi / quá 2 phút)─→ CANCELLED       └─(thất bại / khách hủy / quá 15p)─→ CANCELLED
```

## 1. Schema

```sql
-- ===== order_db =====
CREATE TABLE orders (
  id                 uuid PRIMARY KEY,
  order_code         varchar(32) NOT NULL UNIQUE,
  user_id            uuid NOT NULL,
  idempotency_key    varchar(128) NOT NULL,
  status             varchar(32) NOT NULL,   -- PENDING_STOCK | PENDING_PAYMENT | CONFIRMED | CANCELLED
  payment_method     varchar(16) NOT NULL,   -- COD | MOMO
  payment_status     varchar(32) NOT NULL,   -- UNPAID | PENDING | PAID | FAILED | REFUND_REQUIRED
  -- snapshot địa chỉ: recipient_name, recipient_phone, province_code/name, ward_code/name, address_line, address_label
  currency           varchar(8) NOT NULL DEFAULT 'VND',
  subtotal_minor     bigint NOT NULL,
  shipping_fee_minor bigint NOT NULL,
  total_amount_minor bigint NOT NULL,
  momo_order_id      varchar(64) UNIQUE,
  momo_request_id    varchar(64),
  momo_trans_id      bigint,
  pay_url            text,
  payment_expires_at timestamptz,
  note               text,
  cancel_reason      varchar(128),
  version            bigint NOT NULL DEFAULT 0,      -- @Version
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, idempotency_key)
);
CREATE INDEX ix_orders_reconcile ON orders (status, created_at, payment_expires_at);
-- order_items: snapshot từng dòng (product_id, product_version, name, variant, giá, số lượng, cart_item_id...)

CREATE TABLE idempotency_records (
  user_id             uuid NOT NULL,
  idempotency_key     varchar(128) NOT NULL,
  request_fingerprint varchar(64) NOT NULL,
  status              varchar(24) NOT NULL,   -- IN_PROGRESS | COMPLETED | FAILED_TERMINAL
  order_id            uuid NOT NULL,
  error_json          jsonb,                  -- chỉ dùng khi FAILED_TERMINAL
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, idempotency_key)
  -- Không có expires_at: key là UUID, không bao giờ được dùng lại hợp lệ
);

CREATE TABLE outbox_events (
  id              uuid PRIMARY KEY,
  event_type      varchar(64) NOT NULL,
  aggregate_id    varchar(64) NOT NULL,
  payload         jsonb NOT NULL,
  attempts        int NOT NULL DEFAULT 0,
  next_attempt_at timestamptz NOT NULL DEFAULT now(),  -- kiêm "hạn thuê" khi relay claim
  published_at    timestamptz,
  last_error      text,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_outbox_unpublished ON outbox_events (next_attempt_at, created_at) WHERE published_at IS NULL;

-- ===== inventory_db =====
-- stock(product_id PK, quantity, reserved, CHECK (reserved >= 0 AND reserved <= quantity))
CREATE TABLE stock_reservations (
  order_id       text PRIMARY KEY,
  status         text NOT NULL CHECK (status IN ('ACTIVE','CONFIRMED','RELEASED')),
  expires_at     timestamptz NOT NULL,
  release_reason text NOT NULL DEFAULT '',
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE stock_reservation_items (
  order_id   text REFERENCES stock_reservations(order_id) ON DELETE CASCADE,
  product_id text NOT NULL,
  quantity   int NOT NULL CHECK (quantity > 0),
  PRIMARY KEY (order_id, product_id)
);
-- Không có processed_events / outbox_events: trạng thái reservation theo order_id đã đủ idempotent

-- ===== cart_db =====
CREATE TABLE processed_events (
  event_id     text PRIMARY KEY,
  event_type   text NOT NULL,
  processed_at timestamptz NOT NULL DEFAULT now()
);
```

## 2. order-service: luồng tạo đơn

```java
public CreateOrderResponse createOrder(CreateOrderCommand cmd) {
    String fp = fingerprint(cmd); // SHA-256 của payload đã chuẩn hóa (sort items, trim chuỗi)

    // ── B1. Replay nhanh nếu key đã tồn tại (chỉ đọc) ──
    IdemRecord existing = idemRepo.find(cmd.userId(), cmd.idempotencyKey());
    if (existing != null) return replay(existing, fp);

    // ── B2. Các bước CHỈ ĐỌC: thất bại ở đây không ghi gì, không cần dọn ──
    authz.require(cmd.userId(), "order.create");                       // gRPC 500ms
    List<LiveProduct> live = catalog.getProducts(cmd.productIds());    // gRPC 2s
    snapshotVerifier.verifyOrThrow(cmd.items(), live);                 // → 409 PRODUCT_SNAPSHOT_MISMATCH
    Pricing pricing = ShippingPolicy.price(cmd.items(), live);         // giá + phí ship tính lại ở server

    // ── B3. TX #1: chiếm key + tạo đơn, cùng một transaction ──
    Order order = tx.execute(() -> {
        UUID orderId = UUID.randomUUID();
        int claimed = sql("""
            INSERT INTO idempotency_records (user_id, idempotency_key, request_fingerprint, status, order_id)
            VALUES (?, ?, ?, 'IN_PROGRESS', ?)
            ON CONFLICT (user_id, idempotency_key) DO NOTHING""",
            cmd.userId(), cmd.idempotencyKey(), fp, orderId);
        if (claimed == 0) return null;   // request trùng vừa thắng; ON CONFLICT tự chờ bên kia commit
        return orderRepo.insert(newOrder(orderId, cmd, live, pricing, PENDING_STOCK));
    });
    if (order == null) {
        return replay(idemRepo.find(cmd.userId(), cmd.idempotencyKey()), fp);
    }

    // ── B4. Giữ kho: NGOÀI transaction, gọi 1 lần, deadline 1.5s ──
    try {
        inventory.reserveStock(order.getId(), order.reservationLines(), now().plus(Duration.ofMinutes(20)));
    } catch (DomainException e) {
        if (e.is("INVENTORY_INSUFFICIENT") || e.is("RESERVATION_ALREADY_RELEASED")) {
            // Biết chắc kho KHÔNG giữ gì cho đơn này → hủy, không cần bù trừ
            steps.cancel(order.getId(), PENDING_STOCK, e.code(), e.metadata(), false);
            throw e;
        }
        return failUncertainReserve(order);
    } catch (Exception e) {
        return failUncertainReserve(order);
    }

    // ── B5a. COD ──
    if (order.getPaymentMethod() == COD) {
        steps.confirmCod(order.getId());
        return toResponse(orderRepo.findById(order.getId()));
    }

    // ── B5b. MoMo: NGOÀI transaction, timeout 3s connect / 10s read ──
    MomoPayment pay;
    try {
        pay = momoGateway.createPayment(order);   // ký HMAC chiều đi
    } catch (Exception e) {
        steps.cancel(order.getId(), PENDING_STOCK, "PAYMENT_GATEWAY_UNAVAILABLE", Map.of(), true);
        throw unavailable("PAYMENT_GATEWAY_UNAVAILABLE");
    }
    steps.markPendingPayment(order.getId(), pay);  // đơn đã bị hủy trước → ném lỗi, KHÔNG trả payUrl
    return toResponse(orderRepo.findById(order.getId()));
}

private CreateOrderResponse failUncertainReserve(Order order) {
    // Timeout / đứt mạng: không biết kho đã giữ chưa → hủy + bù trừ (tombstone lo phần gói đến muộn)
    steps.cancel(order.getId(), PENDING_STOCK, "INVENTORY_UNAVAILABLE", Map.of(), true);
    throw unavailable("INVENTORY_UNAVAILABLE");
}

private CreateOrderResponse replay(IdemRecord r, String fp) {
    // this is a replay of a previous request with the same idempotency key, although the request fingerprint may differ (e.g. user changed note or address), appear when the user resubmits the same request with the same idempotency key but different content, or the frontend doesn't handle it well.
    if (!r.getFingerprint().equals(fp)) throw conflict("IDEMPOTENCY_KEY_REUSED");
    return switch (r.getStatus()) {
        case IN_PROGRESS     -> throw conflict("REQUEST_IN_PROGRESS");
        case COMPLETED       -> toResponse(orderRepo.findById(r.getOrderId())); // ĐỌC LẠI trạng thái hiện tại
        case FAILED_TERMINAL -> throw DomainException.fromJson(r.getErrorJson());
    };
}

private CreateOrderResponse toResponse(Order o) {
    // payUrl chỉ trả khi đơn còn PENDING_PAYMENT và chưa hết hạn → không bao giờ replay link của đơn đã hủy
    String payUrl = (o.getStatus() == PENDING_PAYMENT && o.getPaymentExpiresAt().isAfter(now())) ? o.getPayUrl() : null;
    return new CreateOrderResponse(toDetail(o), payUrl);
}
```

## 3. order-service: các bước chuyển trạng thái

Mỗi phương thức dưới đây là một transaction ngắn, khóa dòng và kiểm trạng thái nguồn trước khi đổi.

```java
@Service
class OrderTransactionStepService {

    @Transactional
    void cancel(UUID id, OrderStatus expected, String errorCode, Map<String,String> meta, boolean compensate) {
        Order o = orderRepo.findByIdForUpdate(id);
        if (o.getStatus() != expected) return;            // bên khác (job/callback) đã chốt trước, không đè
        o.setStatus(CANCELLED);
        o.setCancelReason(errorCode);
        if (o.getPaymentMethod() == MOMO) o.setPaymentStatus(FAILED);
        if (compensate) outbox.add("ReservationReleaseRequested", o, reason(errorCode));
        idem.markFailedTerminal(o.getUserId(), o.getIdempotencyKey(), errorCode, meta);
    }

    @Transactional
    void confirmCod(UUID id) {
        Order o = orderRepo.findByIdForUpdate(id);
        if (o.getStatus() != PENDING_STOCK) throw unavailable("ORDER_PROCESSING_TIMEOUT"); // job đã hủy và bù trừ rồi
        o.setStatus(CONFIRMED);
        o.setPaymentStatus(UNPAID);
        outbox.add("OrderConfirmed", o, itemsPayload(o));
        idem.markCompleted(o.getUserId(), o.getIdempotencyKey());
    }

    @Transactional
    void markPendingPayment(UUID id, MomoPayment pay) {
        Order o = orderRepo.findByIdForUpdate(id);
        if (o.getStatus() != PENDING_STOCK) throw unavailable("PAYMENT_GATEWAY_UNAVAILABLE"); // KHÔNG trả payUrl
        o.setStatus(PENDING_PAYMENT);
        o.setPaymentStatus(PENDING);
        o.setMomo(pay.momoOrderId(), pay.requestId(), pay.payUrl());
        o.setPaymentExpiresAt(now().plus(paymentTtl));    // 15 phút
        idem.markCompleted(o.getUserId(), o.getIdempotencyKey());
    }

    @Transactional
    void expirePayment(UUID id) {
        Order o = orderRepo.findByIdForUpdate(id);
        if (o.getStatus() != PENDING_PAYMENT || o.getPaymentExpiresAt().isAfter(now())) return;
        o.setStatus(CANCELLED);
        o.setPaymentStatus(FAILED);
        o.setCancelReason("PAYMENT_EXPIRED");
        outbox.add("ReservationReleaseRequested", o, reason("PAYMENT_EXPIRED"));
        // Bản ghi idempotency giữ COMPLETED: replay luôn đọc lại đơn nên sẽ thấy CANCELLED, không thấy payUrl
    }
}
```

## 4. order-service: callback MoMo, khách hủy đơn, job đối soát, outbox relay

```java
// ===== Callback MoMo (redirect và IPN dùng chung) =====
public CallbackResult handleMomoCallback(MomoCallback cb) {
    if (!signatureVerifier.verify(cb)) throw invalidArg("INVALID_MOMO_SIGNATURE"); // HMAC, MessageDigest.isEqual

    return tx.execute(() -> {
        Order o = orderRepo.findByMomoOrderIdForUpdate(cb.orderId());
        if (o == null) throw notFound("ORDER_NOT_FOUND");
        if (cb.amount() != o.getTotalAmountMinor()) throw invalidArg("INVALID_MOMO_SIGNATURE");

        boolean paid = cb.resultCode() == 0;
        switch (o.getStatus()) {
            case PENDING_PAYMENT -> {
                if (paid) {
                    o.setStatus(CONFIRMED);
                    o.setPaymentStatus(PAID);
                    o.setMomoTransId(cb.transId());
                    outbox.add("OrderConfirmed", o, itemsPayload(o));
                } else {
                    o.setStatus(CANCELLED);
                    o.setPaymentStatus(FAILED);
                    o.setCancelReason("MOMO_RESULT_" + cb.resultCode());
                    outbox.add("ReservationReleaseRequested", o, reason("MOMO_FAILED"));
                }
            }
            case CANCELLED -> {
                // Thanh toán đến muộn: KHÔNG mở lại đơn (kho đã nhả cho người khác)
                if (paid && o.getPaymentStatus() != REFUND_REQUIRED) {
                    o.setPaymentStatus(REFUND_REQUIRED);
                    o.setMomoTransId(cb.transId());
                    log.warn("LATE_PAYMENT_REFUND_REQUIRED order={} transId={}", o.getId(), cb.transId());
                }
            }
            case CONFIRMED, PENDING_STOCK -> { /* callback lặp hoặc bất thường: không làm gì */ }
        }
        return CallbackResult.of(o, cb.resultCode());
    });
}

// ===== Khách hủy đơn =====
@Transactional
public OrderDetail cancelOrder(UUID userId, UUID orderId, String reason) {
    Order o = orderRepo.findByIdAndUserIdForUpdate(orderId, userId);
    if (o == null) throw notFound("ORDER_NOT_FOUND");
    if (o.getStatus() == CANCELLED) return toDetail(o);                 // bấm hủy lặp: trả lại kết quả cũ
    if (o.getStatus() != PENDING_PAYMENT) throw failedPrecondition("ORDER_NOT_CANCELLABLE");
    o.setStatus(CANCELLED);
    o.setPaymentStatus(FAILED);
    o.setCancelReason("USER_CANCELLED");
    outbox.add("ReservationReleaseRequested", o, reason("USER_CANCELLED"));
    return toDetail(o);
}

// ===== Job đối soát =====
@Scheduled(fixedDelayString = "${order.reconcile.interval:30s}")
public void reconcile() {
    // SELECT không khóa; mỗi bước tự khóa và kiểm lại → chạy nhiều instance vẫn an toàn
    for (UUID id : orderRepo.findIds("status = 'PENDING_STOCK' AND created_at < now() - interval '2 minutes'", 100)) {
        steps.cancel(id, PENDING_STOCK, "ORDER_PROCESSING_TIMEOUT", Map.of(), true); // kèm idem → FAILED_TERMINAL
    }
    for (UUID id : orderRepo.findIds("status = 'PENDING_PAYMENT' AND payment_expires_at < now()", 100)) {
        steps.expirePayment(id);
    }
}

// ===== Outbox relay: claim có hạn thuê, không giữ transaction qua RabbitMQ =====
@Scheduled(fixedDelayString = "${order.outbox.interval:1s}")
public void relayOutbox() {
    List<OutboxEvent> batch = tx.execute(() -> sql("""
        UPDATE outbox_events SET next_attempt_at = now() + interval '15 seconds'
        WHERE id IN (SELECT id FROM outbox_events
                     WHERE published_at IS NULL AND next_attempt_at <= now()
                     ORDER BY created_at LIMIT 50
                     FOR UPDATE SKIP LOCKED)
        RETURNING *"""));                                    // TX ngắn #1, commit ngay

    for (OutboxEvent e : batch) {
        try {
            rabbit.publishAndWaitConfirm("ecommerce.events", routingKey(e), envelope(e), Duration.ofSeconds(3));
            tx.execute(() -> sql("UPDATE outbox_events SET published_at = now() WHERE id = ?", e.getId()));
        } catch (Exception ex) {
            tx.execute(() -> sql("""
                UPDATE outbox_events SET attempts = attempts + 1, last_error = ?,
                       next_attempt_at = now() + ? WHERE id = ?""",
                ex.getMessage(), backoff(e.getAttempts()), e.getId()));
        }
    }
}
```

## 5. inventory-service (Go)

```go
// ===== gRPC ReserveStock =====
func (s *Store) ReserveStock(ctx context.Context, orderID string, items []domain.ReservationLine, expiresAt time.Time) (string, error) {
    lines := aggregateByProductAndSortASC(items) // 2 Đen + 2 Trắng cùng p-1 → {p-1: 4}; sort chống deadlock
    var result string
    err := s.inTx(ctx, func(tx pgx.Tx) error {
        // INSERT trước: nếu gói release đang chèn tombstone song song, câu này tự chờ bên kia commit
        tag, err := tx.Exec(ctx, `
            INSERT INTO stock_reservations (order_id, status, expires_at) VALUES ($1, 'ACTIVE', $2)
            ON CONFLICT (order_id) DO NOTHING`, orderID, expiresAt)
        if err != nil { return err }

        if tag.RowsAffected() == 0 {
            var st string
            if err := tx.QueryRow(ctx,
                `SELECT status FROM stock_reservations WHERE order_id = $1 FOR UPDATE`, orderID).Scan(&st); err != nil {
                return err
            }
            if st == "RELEASED" { return domain.ErrReservationAlreadyReleased } // gặp tombstone
            result = st                                                         // ACTIVE/CONFIRMED: gọi lặp
            return nil
        }

        for _, l := range lines {
            tag, err := tx.Exec(ctx, `
                UPDATE stock SET reserved = reserved + $2, updated_at = now()
                WHERE product_id = $1 AND quantity - reserved >= $2`, l.ProductID, l.Quantity)
            if err != nil { return err }
            if tag.RowsAffected() == 0 {
                avail := queryAvailable(ctx, tx, l.ProductID)
                return &domain.InsufficientStockError{ProductID: l.ProductID, Available: avail, Requested: l.Quantity}
                // → ROLLBACK cả phiếu ACTIVE vừa chèn, không rò reserved
            }
            if _, err := tx.Exec(ctx,
                `INSERT INTO stock_reservation_items (order_id, product_id, quantity) VALUES ($1, $2, $3)`,
                orderID, l.ProductID, l.Quantity); err != nil {
                return err
            }
        }
        result = "ACTIVE"
        return nil
    })
    return result, err
}

// ===== Consumer: ReservationReleaseRequested =====
func (s *Store) ReleaseReservation(ctx context.Context, tx pgx.Tx, orderID, reason string) error {
    tag, err := tx.Exec(ctx, `
        INSERT INTO stock_reservations (order_id, status, expires_at, release_reason) VALUES ($1, 'RELEASED', now(), $2)
        ON CONFLICT (order_id) DO NOTHING`, orderID, reason)
    if err != nil { return err }
    if tag.RowsAffected() == 1 { return nil } // tombstone: chặn gói ReserveStock đến muộn

    var st string
    if err := tx.QueryRow(ctx,
        `SELECT status FROM stock_reservations WHERE order_id = $1 FOR UPDATE`, orderID).Scan(&st); err != nil {
        return err
    }
    if st != "ACTIVE" { return nil } // đã RELEASED (giao lặp) hoặc CONFIRMED (bất thường): bỏ qua

    for _, it := range loadItemsSortedASC(ctx, tx, orderID) {
        if _, err := tx.Exec(ctx,
            `UPDATE stock SET reserved = reserved - $2, updated_at = now() WHERE product_id = $1`,
            it.ProductID, it.Quantity); err != nil {
            return err
        }
    }
    _, err = tx.Exec(ctx, `
        UPDATE stock_reservations SET status = 'RELEASED', release_reason = $2, updated_at = now()
        WHERE order_id = $1`, orderID, reason)
    return err
}

// ===== Consumer: OrderConfirmed =====
func (s *Store) ConfirmReservation(ctx context.Context, tx pgx.Tx, orderID string) error {
    var st string
    err := tx.QueryRow(ctx,
        `SELECT status FROM stock_reservations WHERE order_id = $1 FOR UPDATE`, orderID).Scan(&st)
    if errors.Is(err, pgx.ErrNoRows) || st == "RELEASED" {
        return ErrReservationAnomaly // không thể xảy ra nếu order-service đúng → để vào DLQ cảnh báo
    }
    if err != nil { return err }
    if st == "CONFIRMED" { return nil } // giao lặp

    for _, it := range loadItemsSortedASC(ctx, tx, orderID) {
        if _, err := tx.Exec(ctx, `
            UPDATE stock SET quantity = quantity - $2, reserved = reserved - $2, updated_at = now()
            WHERE product_id = $1`, it.ProductID, it.Quantity); err != nil {
            return err
        }
    }
    _, err = tx.Exec(ctx,
        `UPDATE stock_reservations SET status = 'CONFIRMED', updated_at = now() WHERE order_id = $1`, orderID)
    return err
}

// ===== Khung consumer dùng chung (inventory và cart) =====
func (c *Consumer) handle(msg amqp.Delivery) {
    evt, err := decodeEnvelope(msg.Body)
    if err != nil { msg.Nack(false, false); return } // message hỏng → DLQ ngay

    err = c.store.InTx(ctx, func(tx pgx.Tx) error {
        tx.Exec(ctx, `SET LOCAL lock_timeout = '2s'`) // không treo connection khi đang flash sale
        switch evt.Type {
        case "OrderConfirmed":              return c.store.ConfirmReservation(ctx, tx, evt.Data.OrderID)
        case "ReservationReleaseRequested": return c.store.ReleaseReservation(ctx, tx, evt.Data.OrderID, evt.Data.Reason)
        default:                            return nil // event lạ: bỏ qua
        }
    }) // lỗi bất kỳ → ROLLBACK toàn bộ; KHÔNG được nuốt lỗi rồi return nil

    switch {
    case err == nil:
        msg.Ack(false)                       // CHỈ ACK sau khi commit
    case deliveryCount(msg) >= 5:
        msg.Nack(false, false)               // → DLQ inventory.order-events.dlq
    default:
        msg.Nack(false, true)                // giao lại
    }
}
```

## 6. cart-service và frontend

```go
// ===== cart-service: OrderConfirmed =====
func (s *Store) RemoveOrderedItems(ctx context.Context, eventID, userID string, items []domain.OrderedItemRef) error {
    return s.inTx(ctx, func(tx pgx.Tx) error {
        // Cart CẦN processed_events: xóa theo product+variant không idempotent
        // (khách vừa thêm lại món đó thì lần giao lặp sẽ xóa nhầm)
        tag, err := tx.Exec(ctx, `
            INSERT INTO processed_events (event_id, event_type) VALUES ($1, 'OrderConfirmed')
            ON CONFLICT (event_id) DO NOTHING`, eventID)
        if err != nil { return err }
        if tag.RowsAffected() == 0 { return nil }

        for _, it := range items {
            if isUUID(it.CartItemID) {
                tx.Exec(ctx, `DELETE FROM cart_items WHERE user_id = $1 AND id = $2`, userID, it.CartItemID)
            } else {
                tx.Exec(ctx, `DELETE FROM cart_items WHERE user_id = $1 AND product_id = $2 AND variant_label = $3`,
                    userID, it.ProductID, it.VariantLabel)
            }
        }
        return nil
    })
}
```

```ts
// ===== ecommerce-fe: OrderPreviewPage =====
const keyRef = useRef(crypto.randomUUID());

// Đổi đầu vào = một ý định đặt hàng mới → key mới
useEffect(() => { keyRef.current = crypto.randomUUID(); }, [addressId, paymentMethod, note]);

async function handleSubmit() {
  setSubmitting(true);                        // khóa nút, chặn bấm đúp ngay trên UI
  try {
    const result = await createOrderAwaitingInProgress(buildPayload(), keyRef.current);
    keyRef.current = crypto.randomUUID();     // đã có kết quả cuối
    if (result.payUrl) {
      redirectToExternalUrl(result.payUrl);
    } else {
      retryLoad();
      navigate(`/don-hang/${result.order.orderId}?placed=1`);
    }
  } catch (err) {
    if (err instanceof ApiError && err.status === null) {
      setRetryHint(true);                     // KHÔNG có response: GIỮ key, bấm lại sẽ replay đúng kết quả thật
    } else {
      keyRef.current = crypto.randomUUID();   // đã có response lỗi cuối cùng → key mới cho lần sau
      if (err instanceof ApiError && err.code === "PRODUCT_SNAPSHOT_MISMATCH") showMismatchBanner(err.errors);
    }
  } finally {
    setSubmitting(false);
  }
}

// REQUEST_IN_PROGRESS = đơn gốc VẪN ĐANG CHẠY → giữ nguyên key và chờ, tuyệt đối không xoay key
async function createOrderAwaitingInProgress(payload: CreateOrderPayload, key: string) {
  for (let i = 0; i < 10; i++) {
    try {
      return await createOrder(payload, key);
    } catch (err) {
      if (err instanceof ApiError && err.code === "REQUEST_IN_PROGRESS") { await sleep(2000); continue; }
      throw err;
    }
  }
  throw new ApiError(null, "REQUEST_IN_PROGRESS_TIMEOUT"); // status null → giữ key, khách bấm lại sau
}
```

## 7. Ma trận edge case

### A. Dữ liệu và tranh chấp kho

| # | Tình huống | Cách xử lý | Ở đâu |
|:-:|---|---|---|
| A1 | Admin đổi giá, tên hoặc ẩn sản phẩm khi khách đang ở trang xem trước | So `version` và các trường snapshot, trả `409 PRODUCT_SNAPSHOT_MISMATCH` kèm chi tiết từng sản phẩm, khóa nút đặt hàng | createOrder B2 |
| A2 | Mua 2 biến thể cùng `product_id` vượt tổng kho (2 Đen + 2 Trắng, kho còn 3) | Gộp theo `product_id` thành 4 > 3, rollback cả phiếu | ReserveStock |
| A3 | 50 người tranh 5 món cuối | `UPDATE ... WHERE quantity - reserved >= $2` khóa dòng tuần tự, cộng `CHECK` làm chốt chặn cuối. Đúng 5 người thắng | ReserveStock |
| A4 | Hai đơn nhiều món khóa ngược thứ tự nhau | Sort `product_id ASC` nên mọi transaction lấy khóa cùng chiều, không deadlock | ReserveStock |
| A5 | Client gửi giá hoặc phí ship giả | Server tính lại toàn bộ từ dữ liệu catalog | createOrder B2 |

### B. Idempotency

| # | Tình huống | Cách xử lý | Ở đâu |
|:-:|---|---|---|
| B1 | Khách bấm đúp | Nút bị khóa khi đang gửi. Nếu vẫn lọt thì cùng key nên chỉ ra 1 đơn | FE và B3 |
| B2 | Hai request cùng key chạy song song | `ON CONFLICT DO NOTHING`: bên thua chờ bên thắng commit rồi vào `replay`. Không có lỗi `23505` | B3 |
| B3 | Mất response (lỗi mạng) | Frontend giữ key, bấm lại nhận đúng kết quả thật | FE và replay |
| B4 | Gửi lại trúng lúc đơn gốc đang chạy | `REQUEST_IN_PROGRESS`: frontend giữ key và chờ, không xoay key (nếu xoay sẽ ra đơn trùng) | FE |
| B5 | Cùng key nhưng đổi nội dung | Fingerprint lệch, trả `409 IDEMPOTENCY_KEY_REUSED` | replay |
| B6 | Khách cố ý mua lại y hệt | Đã nhận response nên frontend dùng key mới, ra đơn thứ 2 đúng ý khách | FE |
| B7 | Kiểm quyền hoặc so snapshot thất bại | Chưa ghi gì nên không có bản ghi mồ côi. Gửi lại thì chạy lại từ đầu | B2 |
| B8 | Sập sau TX #1 (`IN_PROGRESS`) | `REQUEST_IN_PROGRESS` tối đa 2 phút. Sau đó job đổi đơn và bản ghi sang `FAILED_TERMINAL` trong cùng transaction | reconcile + `steps.cancel` |
| B9 | Sập sau khi chốt, trước khi trả response | Bản ghi đã `COMPLETED`, replay trả `201` với đúng đơn đó | replay |
| B10 | Replay đơn MoMo sau khi đơn đã bị hủy | Replay đọc lại đơn hiện tại nên thấy `CANCELLED`, không thấy `payUrl` | `toResponse` |

### C. Saga và gãy mạng

| # | Tình huống | Cách xử lý | Ở đâu |
|:-:|---|---|---|
| C1 | `ReserveStock` báo hết hàng | Hủy đơn, không bù trừ, bản ghi `FAILED_TERMINAL` | B4 |
| C2 | `ReserveStock` timeout (không biết kho đã giữ chưa) | Hủy đơn và ghi outbox `ReservationReleaseRequested` | `failUncertainReserve` |
| C3 | Gói release đến trước gói reserve bị trễ | Release cắm tombstone `RELEASED`, gói reserve đến sau bị từ chối | Release/ReserveStock |
| C4 | Release và reserve trễ chạy song song | Cả hai `INSERT ... ON CONFLICT` trước: một bên thắng, bên kia chờ rồi đọc trạng thái. Không có `23505`, kho luôn khớp | Release/ReserveStock |
| C5 | Sập trước hoặc sau khi gọi `ReserveStock` | Đơn treo `PENDING_STOCK` quá 2 phút thì job hủy và bù trừ (nhả kho hoặc cắm tombstone) | reconcile |
| C6 | Gọi MoMo lỗi | Hủy đơn và bù trừ, giỏ hàng giữ nguyên | B5b |
| C7 | MoMo chậm trong khi job đã hủy đơn | Timeout 3s/10s nhỏ hơn hẳn 2 phút. Nếu vẫn xảy ra thì `markPendingPayment` thấy đơn không còn `PENDING_STOCK`, ném lỗi và không trả `payUrl` | `markPendingPayment` |
| C8 | Request chậm và job hủy trước khi chốt COD | `confirmCod` thấy trạng thái khác nên dừng. Job đã bù trừ rồi | `confirmCod` |
| C9 | Sập sau khi đã `CONFIRMED` | Outbox gửi lại `OrderConfirmed` cho tới khi được ACK, tuyệt đối không hoàn kho | relay |

### D. Callback MoMo

| # | Tình huống | Cách xử lý | Ở đâu |
|:-:|---|---|---|
| D1 | Tự sửa URL thành `resultCode=0` | HMAC sai, trả `400 INVALID_MOMO_SIGNATURE` | callback |
| D2 | Chữ ký đúng nhưng `amount` lệch | Từ chối, trả `400` | callback |
| D3 | Redirect, IPN, khách hủy và job hết hạn đập vào cùng lúc | `FOR UPDATE`: bên đầu tiên chốt, các bên sau thấy trạng thái đã khác nên thoát. Đúng 1 event | callback / cancelOrder / expirePayment |
| D4 | IPN lặp nhiều lần | Đơn đã `CONFIRMED` nên không làm gì | callback |
| D5 | Thanh toán đến sau khi đơn đã hủy | Giữ `CANCELLED`, đặt `REFUND_REQUIRED`, ghi cảnh báo để đối soát thủ công | callback |
| D6 | Khách bỏ trang MoMo giữa chừng | Giỏ hàng giữ nguyên (chỉ xóa khi có `OrderConfirmed`). Sau 15 phút job nhả kho | expirePayment |
| D7 | Khách bấm hủy hai lần | Lần sau trả lại đơn đã hủy, không phát event thứ 2 | cancelOrder |

### E. Outbox và consumer

| # | Tình huống | Cách xử lý | Ở đâu |
|:-:|---|---|---|
| E1 | RabbitMQ sập | Event nằm trong outbox, relay thử lại với backoff. Không mất event | relay |
| E2 | Sập giữa lúc đã publish và trước khi đánh dấu `published_at` | Gửi lặp. Consumer idempotent nên vô hại | relay + consumer |
| E3 | Hai instance relay cùng chạy | `SKIP LOCKED` cùng hạn thuê 15 giây nên không lấy trùng batch | relay |
| E4 | Consumer xử lý lỗi | Rollback toàn bộ, NACK, giao lại | khung consumer |
| E5 | Consumer sập sau commit, trước ACK | Giao lại: inventory thấy trạng thái đã đổi, cart thấy `processed_events`. Bỏ qua và ACK | consumer |
| E6 | Lỗi mãi không hết hoặc message bất thường | Sau 5 lần thì vào DLQ để cảnh báo, không chặn các message khác | khung consumer |
| E7 | Chờ khóa dòng `stock` lâu khi flash sale | `lock_timeout 2s`, lỗi thì NACK và giao lại sau | khung consumer |

## 8. Bất biến để test và để mô phỏng tải kiểm chứng

- Mỗi cặp `(user_id, idempotency_key)` có tối đa 1 đơn.
- Mỗi đơn phát tối đa 1 event cuối (`OrderConfirmed` hoặc `ReservationReleaseRequested`).
- Luôn có `0 ≤ reserved ≤ quantity`. Sau khi hệ thống ổn định: tổng `reserved` bằng tổng số lượng của các reservation `ACTIVE`.
- Đơn `CANCELLED` thì reservation là `RELEASED` hoặc không tồn tại. Đơn `CONFIRMED` thì reservation là `CONFIRMED`.
- Không có transaction nào chứa gRPC, HTTPS hay RabbitMQ.
- Không có `payUrl` nào được trả ra cho một đơn không ở trạng thái `PENDING_PAYMENT`.