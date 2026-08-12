package com.ecommerce.contracts.cart.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@javax.annotation.Generated(
    value = "by gRPC proto compiler (version 1.65.1)",
    comments = "Source: cart/v1/cart.proto")
@io.grpc.stub.annotations.GrpcGenerated
public final class CartServiceGrpc {

  private CartServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "ecommerce.cart.v1.CartService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.cart.v1.GetCartRequest,
      com.ecommerce.contracts.cart.v1.GetCartResponse> getGetCartMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetCart",
      requestType = com.ecommerce.contracts.cart.v1.GetCartRequest.class,
      responseType = com.ecommerce.contracts.cart.v1.GetCartResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.cart.v1.GetCartRequest,
      com.ecommerce.contracts.cart.v1.GetCartResponse> getGetCartMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.cart.v1.GetCartRequest, com.ecommerce.contracts.cart.v1.GetCartResponse> getGetCartMethod;
    if ((getGetCartMethod = CartServiceGrpc.getGetCartMethod) == null) {
      synchronized (CartServiceGrpc.class) {
        if ((getGetCartMethod = CartServiceGrpc.getGetCartMethod) == null) {
          CartServiceGrpc.getGetCartMethod = getGetCartMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.cart.v1.GetCartRequest, com.ecommerce.contracts.cart.v1.GetCartResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetCart"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.cart.v1.GetCartRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.cart.v1.GetCartResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CartServiceMethodDescriptorSupplier("GetCart"))
              .build();
        }
      }
    }
    return getGetCartMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.cart.v1.AddItemRequest,
      com.ecommerce.contracts.cart.v1.AddItemResponse> getAddItemMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "AddItem",
      requestType = com.ecommerce.contracts.cart.v1.AddItemRequest.class,
      responseType = com.ecommerce.contracts.cart.v1.AddItemResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.cart.v1.AddItemRequest,
      com.ecommerce.contracts.cart.v1.AddItemResponse> getAddItemMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.cart.v1.AddItemRequest, com.ecommerce.contracts.cart.v1.AddItemResponse> getAddItemMethod;
    if ((getAddItemMethod = CartServiceGrpc.getAddItemMethod) == null) {
      synchronized (CartServiceGrpc.class) {
        if ((getAddItemMethod = CartServiceGrpc.getAddItemMethod) == null) {
          CartServiceGrpc.getAddItemMethod = getAddItemMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.cart.v1.AddItemRequest, com.ecommerce.contracts.cart.v1.AddItemResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "AddItem"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.cart.v1.AddItemRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.cart.v1.AddItemResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CartServiceMethodDescriptorSupplier("AddItem"))
              .build();
        }
      }
    }
    return getAddItemMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.cart.v1.UpdateItemQuantityRequest,
      com.ecommerce.contracts.cart.v1.UpdateItemQuantityResponse> getUpdateItemQuantityMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "UpdateItemQuantity",
      requestType = com.ecommerce.contracts.cart.v1.UpdateItemQuantityRequest.class,
      responseType = com.ecommerce.contracts.cart.v1.UpdateItemQuantityResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.cart.v1.UpdateItemQuantityRequest,
      com.ecommerce.contracts.cart.v1.UpdateItemQuantityResponse> getUpdateItemQuantityMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.cart.v1.UpdateItemQuantityRequest, com.ecommerce.contracts.cart.v1.UpdateItemQuantityResponse> getUpdateItemQuantityMethod;
    if ((getUpdateItemQuantityMethod = CartServiceGrpc.getUpdateItemQuantityMethod) == null) {
      synchronized (CartServiceGrpc.class) {
        if ((getUpdateItemQuantityMethod = CartServiceGrpc.getUpdateItemQuantityMethod) == null) {
          CartServiceGrpc.getUpdateItemQuantityMethod = getUpdateItemQuantityMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.cart.v1.UpdateItemQuantityRequest, com.ecommerce.contracts.cart.v1.UpdateItemQuantityResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "UpdateItemQuantity"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.cart.v1.UpdateItemQuantityRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.cart.v1.UpdateItemQuantityResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CartServiceMethodDescriptorSupplier("UpdateItemQuantity"))
              .build();
        }
      }
    }
    return getUpdateItemQuantityMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.cart.v1.RemoveItemRequest,
      com.ecommerce.contracts.cart.v1.RemoveItemResponse> getRemoveItemMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "RemoveItem",
      requestType = com.ecommerce.contracts.cart.v1.RemoveItemRequest.class,
      responseType = com.ecommerce.contracts.cart.v1.RemoveItemResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.cart.v1.RemoveItemRequest,
      com.ecommerce.contracts.cart.v1.RemoveItemResponse> getRemoveItemMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.cart.v1.RemoveItemRequest, com.ecommerce.contracts.cart.v1.RemoveItemResponse> getRemoveItemMethod;
    if ((getRemoveItemMethod = CartServiceGrpc.getRemoveItemMethod) == null) {
      synchronized (CartServiceGrpc.class) {
        if ((getRemoveItemMethod = CartServiceGrpc.getRemoveItemMethod) == null) {
          CartServiceGrpc.getRemoveItemMethod = getRemoveItemMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.cart.v1.RemoveItemRequest, com.ecommerce.contracts.cart.v1.RemoveItemResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "RemoveItem"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.cart.v1.RemoveItemRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.cart.v1.RemoveItemResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CartServiceMethodDescriptorSupplier("RemoveItem"))
              .build();
        }
      }
    }
    return getRemoveItemMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.cart.v1.ClearCartRequest,
      com.ecommerce.contracts.cart.v1.ClearCartResponse> getClearCartMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ClearCart",
      requestType = com.ecommerce.contracts.cart.v1.ClearCartRequest.class,
      responseType = com.ecommerce.contracts.cart.v1.ClearCartResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.cart.v1.ClearCartRequest,
      com.ecommerce.contracts.cart.v1.ClearCartResponse> getClearCartMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.cart.v1.ClearCartRequest, com.ecommerce.contracts.cart.v1.ClearCartResponse> getClearCartMethod;
    if ((getClearCartMethod = CartServiceGrpc.getClearCartMethod) == null) {
      synchronized (CartServiceGrpc.class) {
        if ((getClearCartMethod = CartServiceGrpc.getClearCartMethod) == null) {
          CartServiceGrpc.getClearCartMethod = getClearCartMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.cart.v1.ClearCartRequest, com.ecommerce.contracts.cart.v1.ClearCartResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ClearCart"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.cart.v1.ClearCartRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.cart.v1.ClearCartResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CartServiceMethodDescriptorSupplier("ClearCart"))
              .build();
        }
      }
    }
    return getClearCartMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static CartServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<CartServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<CartServiceStub>() {
        @java.lang.Override
        public CartServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new CartServiceStub(channel, callOptions);
        }
      };
    return CartServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static CartServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<CartServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<CartServiceBlockingStub>() {
        @java.lang.Override
        public CartServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new CartServiceBlockingStub(channel, callOptions);
        }
      };
    return CartServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static CartServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<CartServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<CartServiceFutureStub>() {
        @java.lang.Override
        public CartServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new CartServiceFutureStub(channel, callOptions);
        }
      };
    return CartServiceFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     */
    default void getCart(com.ecommerce.contracts.cart.v1.GetCartRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.cart.v1.GetCartResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetCartMethod(), responseObserver);
    }

    /**
     */
    default void addItem(com.ecommerce.contracts.cart.v1.AddItemRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.cart.v1.AddItemResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getAddItemMethod(), responseObserver);
    }

    /**
     */
    default void updateItemQuantity(com.ecommerce.contracts.cart.v1.UpdateItemQuantityRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.cart.v1.UpdateItemQuantityResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getUpdateItemQuantityMethod(), responseObserver);
    }

    /**
     */
    default void removeItem(com.ecommerce.contracts.cart.v1.RemoveItemRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.cart.v1.RemoveItemResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getRemoveItemMethod(), responseObserver);
    }

    /**
     */
    default void clearCart(com.ecommerce.contracts.cart.v1.ClearCartRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.cart.v1.ClearCartResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getClearCartMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service CartService.
   */
  public static abstract class CartServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return CartServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service CartService.
   */
  public static final class CartServiceStub
      extends io.grpc.stub.AbstractAsyncStub<CartServiceStub> {
    private CartServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected CartServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new CartServiceStub(channel, callOptions);
    }

    /**
     */
    public void getCart(com.ecommerce.contracts.cart.v1.GetCartRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.cart.v1.GetCartResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetCartMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void addItem(com.ecommerce.contracts.cart.v1.AddItemRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.cart.v1.AddItemResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getAddItemMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void updateItemQuantity(com.ecommerce.contracts.cart.v1.UpdateItemQuantityRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.cart.v1.UpdateItemQuantityResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getUpdateItemQuantityMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void removeItem(com.ecommerce.contracts.cart.v1.RemoveItemRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.cart.v1.RemoveItemResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getRemoveItemMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void clearCart(com.ecommerce.contracts.cart.v1.ClearCartRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.cart.v1.ClearCartResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getClearCartMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service CartService.
   */
  public static final class CartServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<CartServiceBlockingStub> {
    private CartServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected CartServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new CartServiceBlockingStub(channel, callOptions);
    }

    /**
     */
    public com.ecommerce.contracts.cart.v1.GetCartResponse getCart(com.ecommerce.contracts.cart.v1.GetCartRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetCartMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.cart.v1.AddItemResponse addItem(com.ecommerce.contracts.cart.v1.AddItemRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getAddItemMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.cart.v1.UpdateItemQuantityResponse updateItemQuantity(com.ecommerce.contracts.cart.v1.UpdateItemQuantityRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getUpdateItemQuantityMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.cart.v1.RemoveItemResponse removeItem(com.ecommerce.contracts.cart.v1.RemoveItemRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getRemoveItemMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.cart.v1.ClearCartResponse clearCart(com.ecommerce.contracts.cart.v1.ClearCartRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getClearCartMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service CartService.
   */
  public static final class CartServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<CartServiceFutureStub> {
    private CartServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected CartServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new CartServiceFutureStub(channel, callOptions);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.cart.v1.GetCartResponse> getCart(
        com.ecommerce.contracts.cart.v1.GetCartRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetCartMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.cart.v1.AddItemResponse> addItem(
        com.ecommerce.contracts.cart.v1.AddItemRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getAddItemMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.cart.v1.UpdateItemQuantityResponse> updateItemQuantity(
        com.ecommerce.contracts.cart.v1.UpdateItemQuantityRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getUpdateItemQuantityMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.cart.v1.RemoveItemResponse> removeItem(
        com.ecommerce.contracts.cart.v1.RemoveItemRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getRemoveItemMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.cart.v1.ClearCartResponse> clearCart(
        com.ecommerce.contracts.cart.v1.ClearCartRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getClearCartMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_GET_CART = 0;
  private static final int METHODID_ADD_ITEM = 1;
  private static final int METHODID_UPDATE_ITEM_QUANTITY = 2;
  private static final int METHODID_REMOVE_ITEM = 3;
  private static final int METHODID_CLEAR_CART = 4;

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
        case METHODID_GET_CART:
          serviceImpl.getCart((com.ecommerce.contracts.cart.v1.GetCartRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.cart.v1.GetCartResponse>) responseObserver);
          break;
        case METHODID_ADD_ITEM:
          serviceImpl.addItem((com.ecommerce.contracts.cart.v1.AddItemRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.cart.v1.AddItemResponse>) responseObserver);
          break;
        case METHODID_UPDATE_ITEM_QUANTITY:
          serviceImpl.updateItemQuantity((com.ecommerce.contracts.cart.v1.UpdateItemQuantityRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.cart.v1.UpdateItemQuantityResponse>) responseObserver);
          break;
        case METHODID_REMOVE_ITEM:
          serviceImpl.removeItem((com.ecommerce.contracts.cart.v1.RemoveItemRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.cart.v1.RemoveItemResponse>) responseObserver);
          break;
        case METHODID_CLEAR_CART:
          serviceImpl.clearCart((com.ecommerce.contracts.cart.v1.ClearCartRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.cart.v1.ClearCartResponse>) responseObserver);
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
          getGetCartMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.cart.v1.GetCartRequest,
              com.ecommerce.contracts.cart.v1.GetCartResponse>(
                service, METHODID_GET_CART)))
        .addMethod(
          getAddItemMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.cart.v1.AddItemRequest,
              com.ecommerce.contracts.cart.v1.AddItemResponse>(
                service, METHODID_ADD_ITEM)))
        .addMethod(
          getUpdateItemQuantityMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.cart.v1.UpdateItemQuantityRequest,
              com.ecommerce.contracts.cart.v1.UpdateItemQuantityResponse>(
                service, METHODID_UPDATE_ITEM_QUANTITY)))
        .addMethod(
          getRemoveItemMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.cart.v1.RemoveItemRequest,
              com.ecommerce.contracts.cart.v1.RemoveItemResponse>(
                service, METHODID_REMOVE_ITEM)))
        .addMethod(
          getClearCartMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.cart.v1.ClearCartRequest,
              com.ecommerce.contracts.cart.v1.ClearCartResponse>(
                service, METHODID_CLEAR_CART)))
        .build();
  }

  private static abstract class CartServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    CartServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return com.ecommerce.contracts.cart.v1.CartOuterClass.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("CartService");
    }
  }

  private static final class CartServiceFileDescriptorSupplier
      extends CartServiceBaseDescriptorSupplier {
    CartServiceFileDescriptorSupplier() {}
  }

  private static final class CartServiceMethodDescriptorSupplier
      extends CartServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    CartServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (CartServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new CartServiceFileDescriptorSupplier())
              .addMethod(getGetCartMethod())
              .addMethod(getAddItemMethod())
              .addMethod(getUpdateItemQuantityMethod())
              .addMethod(getRemoveItemMethod())
              .addMethod(getClearCartMethod())
              .build();
        }
      }
    }
    return result;
  }
}
