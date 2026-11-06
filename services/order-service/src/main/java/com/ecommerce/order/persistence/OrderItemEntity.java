package com.ecommerce.order.persistence;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.FetchType;
import jakarta.persistence.Id;
import jakarta.persistence.JoinColumn;
import jakarta.persistence.ManyToOne;
import jakarta.persistence.Table;
import java.util.UUID;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

@Entity
@Table(name = "order_items")
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
@Builder
public class OrderItemEntity {

    @Id
    @Column(name = "id", nullable = false)
    private UUID id;

    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "order_id", nullable = false)
    private OrderEntity order;

    @Column(name = "cart_item_id", nullable = false, length = 64)
    @Builder.Default
    private String cartItemId = "";

    @Column(name = "product_id", nullable = false, length = 128)
    private String productId;

    @Column(name = "sku_id", nullable = false, length = 128)
    private String skuId;

    @Column(name = "product_version", nullable = false)
    private long productVersion;

    @Column(name = "product_name", nullable = false, length = 255)
    private String productName;

    @Column(name = "brand", nullable = false, length = 128)
    @Builder.Default
    private String brand = "";

    @Column(name = "category_name", nullable = false, length = 128)
    @Builder.Default
    private String categoryName = "";

    @Column(name = "variant_label", nullable = false, length = 128)
    @Builder.Default
    private String variantLabel = "";

    @Column(name = "variant_color_hex", nullable = false, length = 32)
    @Builder.Default
    private String variantColorHex = "";

    @Column(name = "image_url", nullable = false, columnDefinition = "text")
    @Builder.Default
    private String imageUrl = "";

    @Column(name = "image_emoji", nullable = false, length = 32)
    @Builder.Default
    private String imageEmoji = "";

    @Column(name = "image_hue", nullable = false, length = 64)
    @Builder.Default
    private String imageHue = "";

    @Column(name = "unit_price_minor", nullable = false)
    private long unitPriceMinor;

    @Column(name = "quantity", nullable = false)
    private int quantity;

    @Column(name = "line_total_minor", nullable = false)
    private long lineTotalMinor;
}
