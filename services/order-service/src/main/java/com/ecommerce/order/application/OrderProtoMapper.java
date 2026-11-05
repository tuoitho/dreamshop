package com.ecommerce.order.application;

import com.ecommerce.contracts.common.v1.Money;
import com.ecommerce.contracts.order.v1.CreateOrderResponse;
import com.ecommerce.contracts.order.v1.OrderDetail;
import com.ecommerce.contracts.order.v1.OrderItem;
import com.ecommerce.contracts.order.v1.ShippingAddressSnapshot;
import com.ecommerce.order.persistence.OrderEntity;
import com.ecommerce.order.persistence.OrderItemEntity;
import java.util.Arrays;

public final class OrderProtoMapper {

    private OrderProtoMapper() {
    }

    public static OrderDetail toOrderDetail(OrderEntity order) {
        String currency = order.getCurrency() == null || order.getCurrency().isBlank() ? "VND" : order.getCurrency();
        OrderDetail.Builder builder = OrderDetail.newBuilder()
                .setOrderId(order.getId().toString())
                .setOrderCode(nullToEmpty(order.getOrderCode()))
                .setUserId(order.getUserId().toString())
                .setStatus(order.getStatus().name())
                .setPaymentMethod(order.getPaymentMethod().name())
                .setPaymentStatus(order.getPaymentStatus().name())
                .setShippingAddress(ShippingAddressSnapshot.newBuilder()
                        .setRecipientName(nullToEmpty(order.getRecipientName()))
                        .setPhone(nullToEmpty(order.getRecipientPhone()))
                        .setProvinceCode(nullToEmpty(order.getProvinceCode()))
                        .setProvinceName(nullToEmpty(order.getProvinceName()))
                        .setWardCode(nullToEmpty(order.getWardCode()))
                        .setWardName(nullToEmpty(order.getWardName()))
                        .setAddressLine(nullToEmpty(order.getAddressLine()))
                        .setLabel(nullToEmpty(order.getAddressLabel()))
                        .build())
                .setSubtotal(money(currency, order.getSubtotalMinor()))
                .setShippingFee(money(currency, order.getShippingFeeMinor()))
                .setTotalAmount(money(currency, order.getTotalAmountMinor()))
                .setNote(nullToEmpty(order.getNote()))
                .setPayUrl(nullToEmpty(order.getPayUrl()))
                .setCancelReason(nullToEmpty(order.getCancelReason()))
                .setPaymentExpiresAt(order.getPaymentExpiresAt() != null ? order.getPaymentExpiresAt().toString() : "")
                .setCreatedAt(order.getCreatedAt() != null ? order.getCreatedAt().toString() : "")
                .setUpdatedAt(order.getUpdatedAt() != null ? order.getUpdatedAt().toString() : "");

        if (order.getItems() != null) {
            for (OrderItemEntity item : order.getItems()) {
                OrderItem.Builder itemBuilder = OrderItem.newBuilder()
                        .setCartItemId(nullToEmpty(item.getCartItemId()))
                        .setProductId(nullToEmpty(item.getProductId()))
                        .setSkuId(nullToEmpty(item.getSkuId()))
                        .setExpectedProductVersion(item.getProductVersion())
                        .setExpectedName(nullToEmpty(item.getProductName()))
                        .setBrand(nullToEmpty(item.getBrand()))
                        .setCategoryName(nullToEmpty(item.getCategoryName()))
                        .setVariantLabel(nullToEmpty(item.getVariantLabel()))
                        .setVariantColorHex(nullToEmpty(item.getVariantColorHex()))
                        .setImageUrl(nullToEmpty(item.getImageUrl()))
                        .setImageEmoji(nullToEmpty(item.getImageEmoji()))
                        .setUnitPrice(money(currency, item.getUnitPriceMinor()))
                        .setQuantity(item.getQuantity())
                        .setLineTotal(money(currency, item.getLineTotalMinor()));
                if (item.getImageHue() != null && !item.getImageHue().isBlank()) {
                    Arrays.stream(item.getImageHue().split(","))
                            .map(String::trim)
                            .filter(s -> !s.isEmpty())
                            .forEach(s -> {
                                try {
                                    itemBuilder.addImageHue(Integer.parseUnsignedInt(s));
                                } catch (NumberFormatException ignored) {
                                }
                            });
                }
                builder.addItems(itemBuilder.build());
            }
        }
        return builder.build();
    }

    public static CreateOrderResponse toCreateOrderResponse(OrderEntity order) {
        OrderDetail detail = toOrderDetail(order);
        return CreateOrderResponse.newBuilder()
                .setOrderId(detail.getOrderId())
                .setStatus(detail.getStatus())
                .setTotalAmount(detail.getTotalAmount())
                .setOrder(detail)
                .setPayUrl(detail.getPayUrl())
                .build();
    }

    private static Money money(String currency, long amountMinor) {
        return Money.newBuilder()
                .setCurrency(currency)
                .setAmountMinor(amountMinor)
                .build();
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }
}
