package com.ecommerce.contracts.payment.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@javax.annotation.Generated(
    value = "by gRPC proto compiler (version 1.65.1)",
    comments = "Source: payment/v1/payment.proto")
@io.grpc.stub.annotations.GrpcGenerated
public final class PaymentServiceGrpc {

  private PaymentServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "ecommerce.payment.v1.PaymentService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.payment.v1.CreatePaymentSessionRequest,
      com.ecommerce.contracts.payment.v1.CreatePaymentSessionResponse> getCreatePaymentSessionMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "CreatePaymentSession",
      requestType = com.ecommerce.contracts.payment.v1.CreatePaymentSessionRequest.class,
      responseType = com.ecommerce.contracts.payment.v1.CreatePaymentSessionResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.payment.v1.CreatePaymentSessionRequest,
      com.ecommerce.contracts.payment.v1.CreatePaymentSessionResponse> getCreatePaymentSessionMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.payment.v1.CreatePaymentSessionRequest, com.ecommerce.contracts.payment.v1.CreatePaymentSessionResponse> getCreatePaymentSessionMethod;
    if ((getCreatePaymentSessionMethod = PaymentServiceGrpc.getCreatePaymentSessionMethod) == null) {
      synchronized (PaymentServiceGrpc.class) {
        if ((getCreatePaymentSessionMethod = PaymentServiceGrpc.getCreatePaymentSessionMethod) == null) {
          PaymentServiceGrpc.getCreatePaymentSessionMethod = getCreatePaymentSessionMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.payment.v1.CreatePaymentSessionRequest, com.ecommerce.contracts.payment.v1.CreatePaymentSessionResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "CreatePaymentSession"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.payment.v1.CreatePaymentSessionRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.payment.v1.CreatePaymentSessionResponse.getDefaultInstance()))
              .setSchemaDescriptor(new PaymentServiceMethodDescriptorSupplier("CreatePaymentSession"))
              .build();
        }
      }
    }
    return getCreatePaymentSessionMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.payment.v1.HandleMomoCallbackRequest,
      com.ecommerce.contracts.payment.v1.HandleMomoCallbackResponse> getHandleMomoCallbackMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "HandleMomoCallback",
      requestType = com.ecommerce.contracts.payment.v1.HandleMomoCallbackRequest.class,
      responseType = com.ecommerce.contracts.payment.v1.HandleMomoCallbackResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.payment.v1.HandleMomoCallbackRequest,
      com.ecommerce.contracts.payment.v1.HandleMomoCallbackResponse> getHandleMomoCallbackMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.payment.v1.HandleMomoCallbackRequest, com.ecommerce.contracts.payment.v1.HandleMomoCallbackResponse> getHandleMomoCallbackMethod;
    if ((getHandleMomoCallbackMethod = PaymentServiceGrpc.getHandleMomoCallbackMethod) == null) {
      synchronized (PaymentServiceGrpc.class) {
        if ((getHandleMomoCallbackMethod = PaymentServiceGrpc.getHandleMomoCallbackMethod) == null) {
          PaymentServiceGrpc.getHandleMomoCallbackMethod = getHandleMomoCallbackMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.payment.v1.HandleMomoCallbackRequest, com.ecommerce.contracts.payment.v1.HandleMomoCallbackResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "HandleMomoCallback"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.payment.v1.HandleMomoCallbackRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.payment.v1.HandleMomoCallbackResponse.getDefaultInstance()))
              .setSchemaDescriptor(new PaymentServiceMethodDescriptorSupplier("HandleMomoCallback"))
              .build();
        }
      }
    }
    return getHandleMomoCallbackMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static PaymentServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PaymentServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PaymentServiceStub>() {
        @java.lang.Override
        public PaymentServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PaymentServiceStub(channel, callOptions);
        }
      };
    return PaymentServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static PaymentServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PaymentServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PaymentServiceBlockingStub>() {
        @java.lang.Override
        public PaymentServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PaymentServiceBlockingStub(channel, callOptions);
        }
      };
    return PaymentServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static PaymentServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PaymentServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PaymentServiceFutureStub>() {
        @java.lang.Override
        public PaymentServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PaymentServiceFutureStub(channel, callOptions);
        }
      };
    return PaymentServiceFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     */
    default void createPaymentSession(com.ecommerce.contracts.payment.v1.CreatePaymentSessionRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.payment.v1.CreatePaymentSessionResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getCreatePaymentSessionMethod(), responseObserver);
    }

    /**
     */
    default void handleMomoCallback(com.ecommerce.contracts.payment.v1.HandleMomoCallbackRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.payment.v1.HandleMomoCallbackResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getHandleMomoCallbackMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service PaymentService.
   */
  public static abstract class PaymentServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return PaymentServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service PaymentService.
   */
  public static final class PaymentServiceStub
      extends io.grpc.stub.AbstractAsyncStub<PaymentServiceStub> {
    private PaymentServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PaymentServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PaymentServiceStub(channel, callOptions);
    }

    /**
     */
    public void createPaymentSession(com.ecommerce.contracts.payment.v1.CreatePaymentSessionRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.payment.v1.CreatePaymentSessionResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getCreatePaymentSessionMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void handleMomoCallback(com.ecommerce.contracts.payment.v1.HandleMomoCallbackRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.payment.v1.HandleMomoCallbackResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getHandleMomoCallbackMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service PaymentService.
   */
  public static final class PaymentServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<PaymentServiceBlockingStub> {
    private PaymentServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PaymentServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PaymentServiceBlockingStub(channel, callOptions);
    }

    /**
     */
    public com.ecommerce.contracts.payment.v1.CreatePaymentSessionResponse createPaymentSession(com.ecommerce.contracts.payment.v1.CreatePaymentSessionRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getCreatePaymentSessionMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.payment.v1.HandleMomoCallbackResponse handleMomoCallback(com.ecommerce.contracts.payment.v1.HandleMomoCallbackRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getHandleMomoCallbackMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service PaymentService.
   */
  public static final class PaymentServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<PaymentServiceFutureStub> {
    private PaymentServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PaymentServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PaymentServiceFutureStub(channel, callOptions);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.payment.v1.CreatePaymentSessionResponse> createPaymentSession(
        com.ecommerce.contracts.payment.v1.CreatePaymentSessionRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getCreatePaymentSessionMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.payment.v1.HandleMomoCallbackResponse> handleMomoCallback(
        com.ecommerce.contracts.payment.v1.HandleMomoCallbackRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getHandleMomoCallbackMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_CREATE_PAYMENT_SESSION = 0;
  private static final int METHODID_HANDLE_MOMO_CALLBACK = 1;

  private static final class MethodHandlers<Req, Resp> implements
      io.grpc.stub.ServerCalls.UnaryMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ServerStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ClientStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.BidiStreamingMethod<Req, Resp> {
    private final AsyncService serviceImpl;
    private final int methodId;

    MethodHandlers(AsyncService serviceImpl, int methodId) {
      this.serviceImpl = serviceImpl;
      this.methodId = methodId;
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public void invoke(Req request, io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        case METHODID_CREATE_PAYMENT_SESSION:
          serviceImpl.createPaymentSession((com.ecommerce.contracts.payment.v1.CreatePaymentSessionRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.payment.v1.CreatePaymentSessionResponse>) responseObserver);
          break;
        case METHODID_HANDLE_MOMO_CALLBACK:
          serviceImpl.handleMomoCallback((com.ecommerce.contracts.payment.v1.HandleMomoCallbackRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.payment.v1.HandleMomoCallbackResponse>) responseObserver);
          break;
        default:
          throw new AssertionError();
      }
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public io.grpc.stub.StreamObserver<Req> invoke(
        io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        default:
          throw new AssertionError();
      }
    }
  }

  public static final io.grpc.ServerServiceDefinition bindService(AsyncService service) {
    return io.grpc.ServerServiceDefinition.builder(getServiceDescriptor())
        .addMethod(
          getCreatePaymentSessionMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.payment.v1.CreatePaymentSessionRequest,
              com.ecommerce.contracts.payment.v1.CreatePaymentSessionResponse>(
                service, METHODID_CREATE_PAYMENT_SESSION)))
        .addMethod(
          getHandleMomoCallbackMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.payment.v1.HandleMomoCallbackRequest,
              com.ecommerce.contracts.payment.v1.HandleMomoCallbackResponse>(
                service, METHODID_HANDLE_MOMO_CALLBACK)))
        .build();
  }

  private static abstract class PaymentServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    PaymentServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return com.ecommerce.contracts.payment.v1.Payment.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("PaymentService");
    }
  }

  private static final class PaymentServiceFileDescriptorSupplier
      extends PaymentServiceBaseDescriptorSupplier {
    PaymentServiceFileDescriptorSupplier() {}
  }

  private static final class PaymentServiceMethodDescriptorSupplier
      extends PaymentServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    PaymentServiceMethodDescriptorSupplier(java.lang.String methodName) {
      this.methodName = methodName;
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.MethodDescriptor getMethodDescriptor() {
      return getServiceDescriptor().findMethodByName(methodName);
    }
  }

  private static volatile io.grpc.ServiceDescriptor serviceDescriptor;

  public static io.grpc.ServiceDescriptor getServiceDescriptor() {
    io.grpc.ServiceDescriptor result = serviceDescriptor;
    if (result == null) {
      synchronized (PaymentServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new PaymentServiceFileDescriptorSupplier())
              .addMethod(getCreatePaymentSessionMethod())
              .addMethod(getHandleMomoCallbackMethod())
              .build();
        }
      }
    }
    return result;
  }
}
