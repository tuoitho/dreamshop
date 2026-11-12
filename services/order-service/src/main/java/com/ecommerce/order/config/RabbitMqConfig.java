package com.ecommerce.order.config;

import org.springframework.amqp.core.Binding;
import org.springframework.amqp.core.BindingBuilder;
import org.springframework.amqp.core.Queue;
import org.springframework.amqp.core.QueueBuilder;
import org.springframework.amqp.core.TopicExchange;
import org.springframework.amqp.rabbit.connection.ConnectionFactory;
import org.springframework.amqp.rabbit.core.RabbitAdmin;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
public class RabbitMqConfig {
    public static final String EXCHANGE_NAME = "ecommerce.events";
    public static final String DLX_NAME = "ecommerce.events.dlx";
    public static final String PAYMENT_EVENTS_QUEUE = "order.payment-events";
    public static final String PAYMENT_EVENTS_DLQ = "order.payment-events.dlq";

    public static final String RK_ORDER_CONFIRMED = "order.confirmed";
    public static final String RK_ORDER_PLACED = "order.placed";
    public static final String RK_STOCK_RESTOCK_REQUESTED = "order.stock.restock_requested";
    public static final String RK_RESERVATION_RELEASE_REQUESTED = "order.reservation.release_requested";
    public static final String RK_PAYMENT_CANCEL_REQUESTED = "order.payment.cancel_requested";
    public static final String RK_PAYMENT_REFUND_REQUESTED = "order.payment.refund_requested";
    public static final String RK_PAYMENT_SUCCEEDED = "payment.succeeded";
    public static final String RK_PAYMENT_FAILED = "payment.failed";

    @Bean
    public RabbitAdmin rabbitAdmin(ConnectionFactory connectionFactory) {
        return new RabbitAdmin(connectionFactory);
    }

    @Bean
    public TopicExchange eventsExchange() {
        return new TopicExchange(EXCHANGE_NAME, true, false);
    }

    @Bean
    public TopicExchange deadLetterExchange() {
        return new TopicExchange(DLX_NAME, true, false);
    }

    @Bean
    public Queue paymentEventsDlq() {
        return QueueBuilder.durable(PAYMENT_EVENTS_DLQ).build();
    }

    @Bean
    public Binding paymentEventsDlqBinding(
            @Qualifier("paymentEventsDlq") Queue paymentEventsDlq,
            @Qualifier("deadLetterExchange") TopicExchange deadLetterExchange) {
        return BindingBuilder.bind(paymentEventsDlq)
                .to(deadLetterExchange)
                .with(PAYMENT_EVENTS_QUEUE);
    }

    @Bean
    public Queue paymentEventsQueue() {
        return QueueBuilder.durable(PAYMENT_EVENTS_QUEUE)
                .withArgument("x-dead-letter-exchange", DLX_NAME)
                .withArgument("x-dead-letter-routing-key", PAYMENT_EVENTS_QUEUE)
                .build();
    }

    @Bean
    public Binding paymentSucceededBinding(
            @Qualifier("paymentEventsQueue") Queue paymentEventsQueue,
            @Qualifier("eventsExchange") TopicExchange eventsExchange) {
        return BindingBuilder.bind(paymentEventsQueue)
                .to(eventsExchange)
                .with(RK_PAYMENT_SUCCEEDED);
    }

    @Bean
    public Binding paymentFailedBinding(
            @Qualifier("paymentEventsQueue") Queue paymentEventsQueue,
            @Qualifier("eventsExchange") TopicExchange eventsExchange) {
        return BindingBuilder.bind(paymentEventsQueue)
                .to(eventsExchange)
                .with(RK_PAYMENT_FAILED);
    }
}
