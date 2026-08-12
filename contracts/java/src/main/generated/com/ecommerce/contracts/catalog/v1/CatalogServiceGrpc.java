package com.ecommerce.contracts.catalog.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@javax.annotation.Generated(
    value = "by gRPC proto compiler (version 1.65.1)",
    comments = "Source: catalog/v1/catalog.proto")
@io.grpc.stub.annotations.GrpcGenerated
public final class CatalogServiceGrpc {

  private CatalogServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "ecommerce.catalog.v1.CatalogService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.GetProductRequest,
      com.ecommerce.contracts.catalog.v1.GetProductResponse> getGetProductMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetProduct",
      requestType = com.ecommerce.contracts.catalog.v1.GetProductRequest.class,
      responseType = com.ecommerce.contracts.catalog.v1.GetProductResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.GetProductRequest,
      com.ecommerce.contracts.catalog.v1.GetProductResponse> getGetProductMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.GetProductRequest, com.ecommerce.contracts.catalog.v1.GetProductResponse> getGetProductMethod;
    if ((getGetProductMethod = CatalogServiceGrpc.getGetProductMethod) == null) {
      synchronized (CatalogServiceGrpc.class) {
        if ((getGetProductMethod = CatalogServiceGrpc.getGetProductMethod) == null) {
          CatalogServiceGrpc.getGetProductMethod = getGetProductMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.catalog.v1.GetProductRequest, com.ecommerce.contracts.catalog.v1.GetProductResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetProduct"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.GetProductRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.GetProductResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CatalogServiceMethodDescriptorSupplier("GetProduct"))
              .build();
        }
      }
    }
    return getGetProductMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.SearchProductsRequest,
      com.ecommerce.contracts.catalog.v1.SearchProductsResponse> getSearchProductsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "SearchProducts",
      requestType = com.ecommerce.contracts.catalog.v1.SearchProductsRequest.class,
      responseType = com.ecommerce.contracts.catalog.v1.SearchProductsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.SearchProductsRequest,
      com.ecommerce.contracts.catalog.v1.SearchProductsResponse> getSearchProductsMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.SearchProductsRequest, com.ecommerce.contracts.catalog.v1.SearchProductsResponse> getSearchProductsMethod;
    if ((getSearchProductsMethod = CatalogServiceGrpc.getSearchProductsMethod) == null) {
      synchronized (CatalogServiceGrpc.class) {
        if ((getSearchProductsMethod = CatalogServiceGrpc.getSearchProductsMethod) == null) {
          CatalogServiceGrpc.getSearchProductsMethod = getSearchProductsMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.catalog.v1.SearchProductsRequest, com.ecommerce.contracts.catalog.v1.SearchProductsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "SearchProducts"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.SearchProductsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.SearchProductsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CatalogServiceMethodDescriptorSupplier("SearchProducts"))
              .build();
        }
      }
    }
    return getSearchProductsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.SoftDeleteProductRequest,
      com.ecommerce.contracts.catalog.v1.SoftDeleteProductResponse> getSoftDeleteProductMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "SoftDeleteProduct",
      requestType = com.ecommerce.contracts.catalog.v1.SoftDeleteProductRequest.class,
      responseType = com.ecommerce.contracts.catalog.v1.SoftDeleteProductResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.SoftDeleteProductRequest,
      com.ecommerce.contracts.catalog.v1.SoftDeleteProductResponse> getSoftDeleteProductMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.SoftDeleteProductRequest, com.ecommerce.contracts.catalog.v1.SoftDeleteProductResponse> getSoftDeleteProductMethod;
    if ((getSoftDeleteProductMethod = CatalogServiceGrpc.getSoftDeleteProductMethod) == null) {
      synchronized (CatalogServiceGrpc.class) {
        if ((getSoftDeleteProductMethod = CatalogServiceGrpc.getSoftDeleteProductMethod) == null) {
          CatalogServiceGrpc.getSoftDeleteProductMethod = getSoftDeleteProductMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.catalog.v1.SoftDeleteProductRequest, com.ecommerce.contracts.catalog.v1.SoftDeleteProductResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "SoftDeleteProduct"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.SoftDeleteProductRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.SoftDeleteProductResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CatalogServiceMethodDescriptorSupplier("SoftDeleteProduct"))
              .build();
        }
      }
    }
    return getSoftDeleteProductMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static CatalogServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<CatalogServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<CatalogServiceStub>() {
        @java.lang.Override
        public CatalogServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new CatalogServiceStub(channel, callOptions);
        }
      };
    return CatalogServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static CatalogServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<CatalogServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<CatalogServiceBlockingStub>() {
        @java.lang.Override
        public CatalogServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new CatalogServiceBlockingStub(channel, callOptions);
        }
      };
    return CatalogServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static CatalogServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<CatalogServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<CatalogServiceFutureStub>() {
        @java.lang.Override
        public CatalogServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new CatalogServiceFutureStub(channel, callOptions);
        }
      };
    return CatalogServiceFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     */
    default void getProduct(com.ecommerce.contracts.catalog.v1.GetProductRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.GetProductResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetProductMethod(), responseObserver);
    }

    /**
     */
    default void searchProducts(com.ecommerce.contracts.catalog.v1.SearchProductsRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.SearchProductsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getSearchProductsMethod(), responseObserver);
    }

    /**
     */
    default void softDeleteProduct(com.ecommerce.contracts.catalog.v1.SoftDeleteProductRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.SoftDeleteProductResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getSoftDeleteProductMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service CatalogService.
   */
  public static abstract class CatalogServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return CatalogServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service CatalogService.
   */
  public static final class CatalogServiceStub
      extends io.grpc.stub.AbstractAsyncStub<CatalogServiceStub> {
    private CatalogServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected CatalogServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new CatalogServiceStub(channel, callOptions);
    }

    /**
     */
    public void getProduct(com.ecommerce.contracts.catalog.v1.GetProductRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.GetProductResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetProductMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void searchProducts(com.ecommerce.contracts.catalog.v1.SearchProductsRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.SearchProductsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getSearchProductsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void softDeleteProduct(com.ecommerce.contracts.catalog.v1.SoftDeleteProductRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.SoftDeleteProductResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getSoftDeleteProductMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service CatalogService.
   */
  public static final class CatalogServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<CatalogServiceBlockingStub> {
    private CatalogServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected CatalogServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new CatalogServiceBlockingStub(channel, callOptions);
    }

    /**
     */
    public com.ecommerce.contracts.catalog.v1.GetProductResponse getProduct(com.ecommerce.contracts.catalog.v1.GetProductRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetProductMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.catalog.v1.SearchProductsResponse searchProducts(com.ecommerce.contracts.catalog.v1.SearchProductsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getSearchProductsMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.catalog.v1.SoftDeleteProductResponse softDeleteProduct(com.ecommerce.contracts.catalog.v1.SoftDeleteProductRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getSoftDeleteProductMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service CatalogService.
   */
  public static final class CatalogServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<CatalogServiceFutureStub> {
    private CatalogServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected CatalogServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new CatalogServiceFutureStub(channel, callOptions);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.catalog.v1.GetProductResponse> getProduct(
        com.ecommerce.contracts.catalog.v1.GetProductRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetProductMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.catalog.v1.SearchProductsResponse> searchProducts(
        com.ecommerce.contracts.catalog.v1.SearchProductsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getSearchProductsMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.catalog.v1.SoftDeleteProductResponse> softDeleteProduct(
        com.ecommerce.contracts.catalog.v1.SoftDeleteProductRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getSoftDeleteProductMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_GET_PRODUCT = 0;
  private static final int METHODID_SEARCH_PRODUCTS = 1;
  private static final int METHODID_SOFT_DELETE_PRODUCT = 2;

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
        case METHODID_GET_PRODUCT:
          serviceImpl.getProduct((com.ecommerce.contracts.catalog.v1.GetProductRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.GetProductResponse>) responseObserver);
          break;
        case METHODID_SEARCH_PRODUCTS:
          serviceImpl.searchProducts((com.ecommerce.contracts.catalog.v1.SearchProductsRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.SearchProductsResponse>) responseObserver);
          break;
        case METHODID_SOFT_DELETE_PRODUCT:
          serviceImpl.softDeleteProduct((com.ecommerce.contracts.catalog.v1.SoftDeleteProductRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.SoftDeleteProductResponse>) responseObserver);
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
          getGetProductMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.catalog.v1.GetProductRequest,
              com.ecommerce.contracts.catalog.v1.GetProductResponse>(
                service, METHODID_GET_PRODUCT)))
        .addMethod(
          getSearchProductsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.catalog.v1.SearchProductsRequest,
              com.ecommerce.contracts.catalog.v1.SearchProductsResponse>(
                service, METHODID_SEARCH_PRODUCTS)))
        .addMethod(
          getSoftDeleteProductMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.catalog.v1.SoftDeleteProductRequest,
              com.ecommerce.contracts.catalog.v1.SoftDeleteProductResponse>(
                service, METHODID_SOFT_DELETE_PRODUCT)))
        .build();
  }

  private static abstract class CatalogServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    CatalogServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return com.ecommerce.contracts.catalog.v1.Catalog.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("CatalogService");
    }
  }

  private static final class CatalogServiceFileDescriptorSupplier
      extends CatalogServiceBaseDescriptorSupplier {
    CatalogServiceFileDescriptorSupplier() {}
  }

  private static final class CatalogServiceMethodDescriptorSupplier
      extends CatalogServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    CatalogServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (CatalogServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new CatalogServiceFileDescriptorSupplier())
              .addMethod(getGetProductMethod())
              .addMethod(getSearchProductsMethod())
              .addMethod(getSoftDeleteProductMethod())
              .build();
        }
      }
    }
    return result;
  }
}
