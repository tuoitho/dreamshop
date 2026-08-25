package com.ecommerce.contracts.identity.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@javax.annotation.Generated(
    value = "by gRPC proto compiler (version 1.65.1)",
    comments = "Source: identity/v1/authorization.proto")
@io.grpc.stub.annotations.GrpcGenerated
public final class AuthorizationServiceGrpc {

  private AuthorizationServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "ecommerce.identity.v1.AuthorizationService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.identity.v1.CheckPermissionRequest,
      com.ecommerce.contracts.identity.v1.CheckPermissionResponse> getCheckPermissionMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "CheckPermission",
      requestType = com.ecommerce.contracts.identity.v1.CheckPermissionRequest.class,
      responseType = com.ecommerce.contracts.identity.v1.CheckPermissionResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.identity.v1.CheckPermissionRequest,
      com.ecommerce.contracts.identity.v1.CheckPermissionResponse> getCheckPermissionMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.identity.v1.CheckPermissionRequest, com.ecommerce.contracts.identity.v1.CheckPermissionResponse> getCheckPermissionMethod;
    if ((getCheckPermissionMethod = AuthorizationServiceGrpc.getCheckPermissionMethod) == null) {
      synchronized (AuthorizationServiceGrpc.class) {
        if ((getCheckPermissionMethod = AuthorizationServiceGrpc.getCheckPermissionMethod) == null) {
          AuthorizationServiceGrpc.getCheckPermissionMethod = getCheckPermissionMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.identity.v1.CheckPermissionRequest, com.ecommerce.contracts.identity.v1.CheckPermissionResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "CheckPermission"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.identity.v1.CheckPermissionRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.identity.v1.CheckPermissionResponse.getDefaultInstance()))
              .setSchemaDescriptor(new AuthorizationServiceMethodDescriptorSupplier("CheckPermission"))
              .build();
        }
      }
    }
    return getCheckPermissionMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.identity.v1.CheckPermissionsRequest,
      com.ecommerce.contracts.identity.v1.CheckPermissionsResponse> getCheckPermissionsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "CheckPermissions",
      requestType = com.ecommerce.contracts.identity.v1.CheckPermissionsRequest.class,
      responseType = com.ecommerce.contracts.identity.v1.CheckPermissionsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.identity.v1.CheckPermissionsRequest,
      com.ecommerce.contracts.identity.v1.CheckPermissionsResponse> getCheckPermissionsMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.identity.v1.CheckPermissionsRequest, com.ecommerce.contracts.identity.v1.CheckPermissionsResponse> getCheckPermissionsMethod;
    if ((getCheckPermissionsMethod = AuthorizationServiceGrpc.getCheckPermissionsMethod) == null) {
      synchronized (AuthorizationServiceGrpc.class) {
        if ((getCheckPermissionsMethod = AuthorizationServiceGrpc.getCheckPermissionsMethod) == null) {
          AuthorizationServiceGrpc.getCheckPermissionsMethod = getCheckPermissionsMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.identity.v1.CheckPermissionsRequest, com.ecommerce.contracts.identity.v1.CheckPermissionsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "CheckPermissions"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.identity.v1.CheckPermissionsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.identity.v1.CheckPermissionsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new AuthorizationServiceMethodDescriptorSupplier("CheckPermissions"))
              .build();
        }
      }
    }
    return getCheckPermissionsMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static AuthorizationServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<AuthorizationServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<AuthorizationServiceStub>() {
        @java.lang.Override
        public AuthorizationServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new AuthorizationServiceStub(channel, callOptions);
        }
      };
    return AuthorizationServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static AuthorizationServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<AuthorizationServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<AuthorizationServiceBlockingStub>() {
        @java.lang.Override
        public AuthorizationServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new AuthorizationServiceBlockingStub(channel, callOptions);
        }
      };
    return AuthorizationServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static AuthorizationServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<AuthorizationServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<AuthorizationServiceFutureStub>() {
        @java.lang.Override
        public AuthorizationServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new AuthorizationServiceFutureStub(channel, callOptions);
        }
      };
    return AuthorizationServiceFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     */
    default void checkPermission(com.ecommerce.contracts.identity.v1.CheckPermissionRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.identity.v1.CheckPermissionResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getCheckPermissionMethod(), responseObserver);
    }

    /**
     */
    default void checkPermissions(com.ecommerce.contracts.identity.v1.CheckPermissionsRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.identity.v1.CheckPermissionsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getCheckPermissionsMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service AuthorizationService.
   */
  public static abstract class AuthorizationServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return AuthorizationServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service AuthorizationService.
   */
  public static final class AuthorizationServiceStub
      extends io.grpc.stub.AbstractAsyncStub<AuthorizationServiceStub> {
    private AuthorizationServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected AuthorizationServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new AuthorizationServiceStub(channel, callOptions);
    }

    /**
     */
    public void checkPermission(com.ecommerce.contracts.identity.v1.CheckPermissionRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.identity.v1.CheckPermissionResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getCheckPermissionMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void checkPermissions(com.ecommerce.contracts.identity.v1.CheckPermissionsRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.identity.v1.CheckPermissionsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getCheckPermissionsMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service AuthorizationService.
   */
  public static final class AuthorizationServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<AuthorizationServiceBlockingStub> {
    private AuthorizationServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected AuthorizationServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new AuthorizationServiceBlockingStub(channel, callOptions);
    }

    /**
     */
    public com.ecommerce.contracts.identity.v1.CheckPermissionResponse checkPermission(com.ecommerce.contracts.identity.v1.CheckPermissionRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getCheckPermissionMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.identity.v1.CheckPermissionsResponse checkPermissions(com.ecommerce.contracts.identity.v1.CheckPermissionsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getCheckPermissionsMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service AuthorizationService.
   */
  public static final class AuthorizationServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<AuthorizationServiceFutureStub> {
    private AuthorizationServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected AuthorizationServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new AuthorizationServiceFutureStub(channel, callOptions);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.identity.v1.CheckPermissionResponse> checkPermission(
        com.ecommerce.contracts.identity.v1.CheckPermissionRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getCheckPermissionMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.identity.v1.CheckPermissionsResponse> checkPermissions(
        com.ecommerce.contracts.identity.v1.CheckPermissionsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getCheckPermissionsMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_CHECK_PERMISSION = 0;
  private static final int METHODID_CHECK_PERMISSIONS = 1;

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
        case METHODID_CHECK_PERMISSION:
          serviceImpl.checkPermission((com.ecommerce.contracts.identity.v1.CheckPermissionRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.identity.v1.CheckPermissionResponse>) responseObserver);
          break;
        case METHODID_CHECK_PERMISSIONS:
          serviceImpl.checkPermissions((com.ecommerce.contracts.identity.v1.CheckPermissionsRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.identity.v1.CheckPermissionsResponse>) responseObserver);
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
          getCheckPermissionMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.identity.v1.CheckPermissionRequest,
              com.ecommerce.contracts.identity.v1.CheckPermissionResponse>(
                service, METHODID_CHECK_PERMISSION)))
        .addMethod(
          getCheckPermissionsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.identity.v1.CheckPermissionsRequest,
              com.ecommerce.contracts.identity.v1.CheckPermissionsResponse>(
                service, METHODID_CHECK_PERMISSIONS)))
        .build();
  }

  private static abstract class AuthorizationServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    AuthorizationServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return com.ecommerce.contracts.identity.v1.Authorization.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("AuthorizationService");
    }
  }

  private static final class AuthorizationServiceFileDescriptorSupplier
      extends AuthorizationServiceBaseDescriptorSupplier {
    AuthorizationServiceFileDescriptorSupplier() {}
  }

  private static final class AuthorizationServiceMethodDescriptorSupplier
      extends AuthorizationServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    AuthorizationServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (AuthorizationServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new AuthorizationServiceFileDescriptorSupplier())
              .addMethod(getCheckPermissionMethod())
              .addMethod(getCheckPermissionsMethod())
              .build();
        }
      }
    }
    return result;
  }
}
