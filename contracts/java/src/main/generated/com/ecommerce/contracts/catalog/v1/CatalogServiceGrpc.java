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

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.ListAdminProductsRequest,
      com.ecommerce.contracts.catalog.v1.ListAdminProductsResponse> getListAdminProductsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListAdminProducts",
      requestType = com.ecommerce.contracts.catalog.v1.ListAdminProductsRequest.class,
      responseType = com.ecommerce.contracts.catalog.v1.ListAdminProductsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.ListAdminProductsRequest,
      com.ecommerce.contracts.catalog.v1.ListAdminProductsResponse> getListAdminProductsMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.ListAdminProductsRequest, com.ecommerce.contracts.catalog.v1.ListAdminProductsResponse> getListAdminProductsMethod;
    if ((getListAdminProductsMethod = CatalogServiceGrpc.getListAdminProductsMethod) == null) {
      synchronized (CatalogServiceGrpc.class) {
        if ((getListAdminProductsMethod = CatalogServiceGrpc.getListAdminProductsMethod) == null) {
          CatalogServiceGrpc.getListAdminProductsMethod = getListAdminProductsMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.catalog.v1.ListAdminProductsRequest, com.ecommerce.contracts.catalog.v1.ListAdminProductsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListAdminProducts"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.ListAdminProductsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.ListAdminProductsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CatalogServiceMethodDescriptorSupplier("ListAdminProducts"))
              .build();
        }
      }
    }
    return getListAdminProductsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsRequest,
      com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsResponse> getListInventoryCatalogItemsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListInventoryCatalogItems",
      requestType = com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsRequest.class,
      responseType = com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsRequest,
      com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsResponse> getListInventoryCatalogItemsMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsRequest, com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsResponse> getListInventoryCatalogItemsMethod;
    if ((getListInventoryCatalogItemsMethod = CatalogServiceGrpc.getListInventoryCatalogItemsMethod) == null) {
      synchronized (CatalogServiceGrpc.class) {
        if ((getListInventoryCatalogItemsMethod = CatalogServiceGrpc.getListInventoryCatalogItemsMethod) == null) {
          CatalogServiceGrpc.getListInventoryCatalogItemsMethod = getListInventoryCatalogItemsMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsRequest, com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListInventoryCatalogItems"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CatalogServiceMethodDescriptorSupplier("ListInventoryCatalogItems"))
              .build();
        }
      }
    }
    return getListInventoryCatalogItemsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.GetAdminProductRequest,
      com.ecommerce.contracts.catalog.v1.GetAdminProductResponse> getGetAdminProductMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetAdminProduct",
      requestType = com.ecommerce.contracts.catalog.v1.GetAdminProductRequest.class,
      responseType = com.ecommerce.contracts.catalog.v1.GetAdminProductResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.GetAdminProductRequest,
      com.ecommerce.contracts.catalog.v1.GetAdminProductResponse> getGetAdminProductMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.GetAdminProductRequest, com.ecommerce.contracts.catalog.v1.GetAdminProductResponse> getGetAdminProductMethod;
    if ((getGetAdminProductMethod = CatalogServiceGrpc.getGetAdminProductMethod) == null) {
      synchronized (CatalogServiceGrpc.class) {
        if ((getGetAdminProductMethod = CatalogServiceGrpc.getGetAdminProductMethod) == null) {
          CatalogServiceGrpc.getGetAdminProductMethod = getGetAdminProductMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.catalog.v1.GetAdminProductRequest, com.ecommerce.contracts.catalog.v1.GetAdminProductResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetAdminProduct"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.GetAdminProductRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.GetAdminProductResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CatalogServiceMethodDescriptorSupplier("GetAdminProduct"))
              .build();
        }
      }
    }
    return getGetAdminProductMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.CreateProductRequest,
      com.ecommerce.contracts.catalog.v1.CreateProductResponse> getCreateProductMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "CreateProduct",
      requestType = com.ecommerce.contracts.catalog.v1.CreateProductRequest.class,
      responseType = com.ecommerce.contracts.catalog.v1.CreateProductResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.CreateProductRequest,
      com.ecommerce.contracts.catalog.v1.CreateProductResponse> getCreateProductMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.CreateProductRequest, com.ecommerce.contracts.catalog.v1.CreateProductResponse> getCreateProductMethod;
    if ((getCreateProductMethod = CatalogServiceGrpc.getCreateProductMethod) == null) {
      synchronized (CatalogServiceGrpc.class) {
        if ((getCreateProductMethod = CatalogServiceGrpc.getCreateProductMethod) == null) {
          CatalogServiceGrpc.getCreateProductMethod = getCreateProductMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.catalog.v1.CreateProductRequest, com.ecommerce.contracts.catalog.v1.CreateProductResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "CreateProduct"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.CreateProductRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.CreateProductResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CatalogServiceMethodDescriptorSupplier("CreateProduct"))
              .build();
        }
      }
    }
    return getCreateProductMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.UpdateProductRequest,
      com.ecommerce.contracts.catalog.v1.UpdateProductResponse> getUpdateProductMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "UpdateProduct",
      requestType = com.ecommerce.contracts.catalog.v1.UpdateProductRequest.class,
      responseType = com.ecommerce.contracts.catalog.v1.UpdateProductResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.UpdateProductRequest,
      com.ecommerce.contracts.catalog.v1.UpdateProductResponse> getUpdateProductMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.UpdateProductRequest, com.ecommerce.contracts.catalog.v1.UpdateProductResponse> getUpdateProductMethod;
    if ((getUpdateProductMethod = CatalogServiceGrpc.getUpdateProductMethod) == null) {
      synchronized (CatalogServiceGrpc.class) {
        if ((getUpdateProductMethod = CatalogServiceGrpc.getUpdateProductMethod) == null) {
          CatalogServiceGrpc.getUpdateProductMethod = getUpdateProductMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.catalog.v1.UpdateProductRequest, com.ecommerce.contracts.catalog.v1.UpdateProductResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "UpdateProduct"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.UpdateProductRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.UpdateProductResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CatalogServiceMethodDescriptorSupplier("UpdateProduct"))
              .build();
        }
      }
    }
    return getUpdateProductMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadRequest,
      com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadResponse> getCreateProductMediaUploadMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "CreateProductMediaUpload",
      requestType = com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadRequest.class,
      responseType = com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadRequest,
      com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadResponse> getCreateProductMediaUploadMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadRequest, com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadResponse> getCreateProductMediaUploadMethod;
    if ((getCreateProductMediaUploadMethod = CatalogServiceGrpc.getCreateProductMediaUploadMethod) == null) {
      synchronized (CatalogServiceGrpc.class) {
        if ((getCreateProductMediaUploadMethod = CatalogServiceGrpc.getCreateProductMediaUploadMethod) == null) {
          CatalogServiceGrpc.getCreateProductMediaUploadMethod = getCreateProductMediaUploadMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadRequest, com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "CreateProductMediaUpload"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CatalogServiceMethodDescriptorSupplier("CreateProductMediaUpload"))
              .build();
        }
      }
    }
    return getCreateProductMediaUploadMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.ListProductReviewsRequest,
      com.ecommerce.contracts.catalog.v1.ListProductReviewsResponse> getListProductReviewsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListProductReviews",
      requestType = com.ecommerce.contracts.catalog.v1.ListProductReviewsRequest.class,
      responseType = com.ecommerce.contracts.catalog.v1.ListProductReviewsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.ListProductReviewsRequest,
      com.ecommerce.contracts.catalog.v1.ListProductReviewsResponse> getListProductReviewsMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.ListProductReviewsRequest, com.ecommerce.contracts.catalog.v1.ListProductReviewsResponse> getListProductReviewsMethod;
    if ((getListProductReviewsMethod = CatalogServiceGrpc.getListProductReviewsMethod) == null) {
      synchronized (CatalogServiceGrpc.class) {
        if ((getListProductReviewsMethod = CatalogServiceGrpc.getListProductReviewsMethod) == null) {
          CatalogServiceGrpc.getListProductReviewsMethod = getListProductReviewsMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.catalog.v1.ListProductReviewsRequest, com.ecommerce.contracts.catalog.v1.ListProductReviewsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListProductReviews"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.ListProductReviewsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.ListProductReviewsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CatalogServiceMethodDescriptorSupplier("ListProductReviews"))
              .build();
        }
      }
    }
    return getListProductReviewsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.GetReviewEligibilityRequest,
      com.ecommerce.contracts.catalog.v1.GetReviewEligibilityResponse> getGetReviewEligibilityMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetReviewEligibility",
      requestType = com.ecommerce.contracts.catalog.v1.GetReviewEligibilityRequest.class,
      responseType = com.ecommerce.contracts.catalog.v1.GetReviewEligibilityResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.GetReviewEligibilityRequest,
      com.ecommerce.contracts.catalog.v1.GetReviewEligibilityResponse> getGetReviewEligibilityMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.GetReviewEligibilityRequest, com.ecommerce.contracts.catalog.v1.GetReviewEligibilityResponse> getGetReviewEligibilityMethod;
    if ((getGetReviewEligibilityMethod = CatalogServiceGrpc.getGetReviewEligibilityMethod) == null) {
      synchronized (CatalogServiceGrpc.class) {
        if ((getGetReviewEligibilityMethod = CatalogServiceGrpc.getGetReviewEligibilityMethod) == null) {
          CatalogServiceGrpc.getGetReviewEligibilityMethod = getGetReviewEligibilityMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.catalog.v1.GetReviewEligibilityRequest, com.ecommerce.contracts.catalog.v1.GetReviewEligibilityResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetReviewEligibility"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.GetReviewEligibilityRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.GetReviewEligibilityResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CatalogServiceMethodDescriptorSupplier("GetReviewEligibility"))
              .build();
        }
      }
    }
    return getGetReviewEligibilityMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.CreateProductReviewRequest,
      com.ecommerce.contracts.catalog.v1.CreateProductReviewResponse> getCreateProductReviewMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "CreateProductReview",
      requestType = com.ecommerce.contracts.catalog.v1.CreateProductReviewRequest.class,
      responseType = com.ecommerce.contracts.catalog.v1.CreateProductReviewResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.CreateProductReviewRequest,
      com.ecommerce.contracts.catalog.v1.CreateProductReviewResponse> getCreateProductReviewMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.CreateProductReviewRequest, com.ecommerce.contracts.catalog.v1.CreateProductReviewResponse> getCreateProductReviewMethod;
    if ((getCreateProductReviewMethod = CatalogServiceGrpc.getCreateProductReviewMethod) == null) {
      synchronized (CatalogServiceGrpc.class) {
        if ((getCreateProductReviewMethod = CatalogServiceGrpc.getCreateProductReviewMethod) == null) {
          CatalogServiceGrpc.getCreateProductReviewMethod = getCreateProductReviewMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.catalog.v1.CreateProductReviewRequest, com.ecommerce.contracts.catalog.v1.CreateProductReviewResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "CreateProductReview"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.CreateProductReviewRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.CreateProductReviewResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CatalogServiceMethodDescriptorSupplier("CreateProductReview"))
              .build();
        }
      }
    }
    return getCreateProductReviewMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.UpdateProductReviewRequest,
      com.ecommerce.contracts.catalog.v1.UpdateProductReviewResponse> getUpdateProductReviewMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "UpdateProductReview",
      requestType = com.ecommerce.contracts.catalog.v1.UpdateProductReviewRequest.class,
      responseType = com.ecommerce.contracts.catalog.v1.UpdateProductReviewResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.UpdateProductReviewRequest,
      com.ecommerce.contracts.catalog.v1.UpdateProductReviewResponse> getUpdateProductReviewMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.UpdateProductReviewRequest, com.ecommerce.contracts.catalog.v1.UpdateProductReviewResponse> getUpdateProductReviewMethod;
    if ((getUpdateProductReviewMethod = CatalogServiceGrpc.getUpdateProductReviewMethod) == null) {
      synchronized (CatalogServiceGrpc.class) {
        if ((getUpdateProductReviewMethod = CatalogServiceGrpc.getUpdateProductReviewMethod) == null) {
          CatalogServiceGrpc.getUpdateProductReviewMethod = getUpdateProductReviewMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.catalog.v1.UpdateProductReviewRequest, com.ecommerce.contracts.catalog.v1.UpdateProductReviewResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "UpdateProductReview"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.UpdateProductReviewRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.UpdateProductReviewResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CatalogServiceMethodDescriptorSupplier("UpdateProductReview"))
              .build();
        }
      }
    }
    return getUpdateProductReviewMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.DeleteProductReviewRequest,
      com.ecommerce.contracts.catalog.v1.DeleteProductReviewResponse> getDeleteProductReviewMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "DeleteProductReview",
      requestType = com.ecommerce.contracts.catalog.v1.DeleteProductReviewRequest.class,
      responseType = com.ecommerce.contracts.catalog.v1.DeleteProductReviewResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.DeleteProductReviewRequest,
      com.ecommerce.contracts.catalog.v1.DeleteProductReviewResponse> getDeleteProductReviewMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.DeleteProductReviewRequest, com.ecommerce.contracts.catalog.v1.DeleteProductReviewResponse> getDeleteProductReviewMethod;
    if ((getDeleteProductReviewMethod = CatalogServiceGrpc.getDeleteProductReviewMethod) == null) {
      synchronized (CatalogServiceGrpc.class) {
        if ((getDeleteProductReviewMethod = CatalogServiceGrpc.getDeleteProductReviewMethod) == null) {
          CatalogServiceGrpc.getDeleteProductReviewMethod = getDeleteProductReviewMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.catalog.v1.DeleteProductReviewRequest, com.ecommerce.contracts.catalog.v1.DeleteProductReviewResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "DeleteProductReview"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.DeleteProductReviewRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.DeleteProductReviewResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CatalogServiceMethodDescriptorSupplier("DeleteProductReview"))
              .build();
        }
      }
    }
    return getDeleteProductReviewMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadRequest,
      com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadResponse> getCreateReviewMediaUploadMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "CreateReviewMediaUpload",
      requestType = com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadRequest.class,
      responseType = com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadRequest,
      com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadResponse> getCreateReviewMediaUploadMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadRequest, com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadResponse> getCreateReviewMediaUploadMethod;
    if ((getCreateReviewMediaUploadMethod = CatalogServiceGrpc.getCreateReviewMediaUploadMethod) == null) {
      synchronized (CatalogServiceGrpc.class) {
        if ((getCreateReviewMediaUploadMethod = CatalogServiceGrpc.getCreateReviewMediaUploadMethod) == null) {
          CatalogServiceGrpc.getCreateReviewMediaUploadMethod = getCreateReviewMediaUploadMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadRequest, com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "CreateReviewMediaUpload"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CatalogServiceMethodDescriptorSupplier("CreateReviewMediaUpload"))
              .build();
        }
      }
    }
    return getCreateReviewMediaUploadMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.ListAdminReviewsRequest,
      com.ecommerce.contracts.catalog.v1.ListAdminReviewsResponse> getListAdminReviewsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListAdminReviews",
      requestType = com.ecommerce.contracts.catalog.v1.ListAdminReviewsRequest.class,
      responseType = com.ecommerce.contracts.catalog.v1.ListAdminReviewsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.ListAdminReviewsRequest,
      com.ecommerce.contracts.catalog.v1.ListAdminReviewsResponse> getListAdminReviewsMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.ListAdminReviewsRequest, com.ecommerce.contracts.catalog.v1.ListAdminReviewsResponse> getListAdminReviewsMethod;
    if ((getListAdminReviewsMethod = CatalogServiceGrpc.getListAdminReviewsMethod) == null) {
      synchronized (CatalogServiceGrpc.class) {
        if ((getListAdminReviewsMethod = CatalogServiceGrpc.getListAdminReviewsMethod) == null) {
          CatalogServiceGrpc.getListAdminReviewsMethod = getListAdminReviewsMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.catalog.v1.ListAdminReviewsRequest, com.ecommerce.contracts.catalog.v1.ListAdminReviewsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListAdminReviews"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.ListAdminReviewsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.ListAdminReviewsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CatalogServiceMethodDescriptorSupplier("ListAdminReviews"))
              .build();
        }
      }
    }
    return getListAdminReviewsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.SetReviewVisibilityRequest,
      com.ecommerce.contracts.catalog.v1.SetReviewVisibilityResponse> getSetReviewVisibilityMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "SetReviewVisibility",
      requestType = com.ecommerce.contracts.catalog.v1.SetReviewVisibilityRequest.class,
      responseType = com.ecommerce.contracts.catalog.v1.SetReviewVisibilityResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.SetReviewVisibilityRequest,
      com.ecommerce.contracts.catalog.v1.SetReviewVisibilityResponse> getSetReviewVisibilityMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.SetReviewVisibilityRequest, com.ecommerce.contracts.catalog.v1.SetReviewVisibilityResponse> getSetReviewVisibilityMethod;
    if ((getSetReviewVisibilityMethod = CatalogServiceGrpc.getSetReviewVisibilityMethod) == null) {
      synchronized (CatalogServiceGrpc.class) {
        if ((getSetReviewVisibilityMethod = CatalogServiceGrpc.getSetReviewVisibilityMethod) == null) {
          CatalogServiceGrpc.getSetReviewVisibilityMethod = getSetReviewVisibilityMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.catalog.v1.SetReviewVisibilityRequest, com.ecommerce.contracts.catalog.v1.SetReviewVisibilityResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "SetReviewVisibility"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.SetReviewVisibilityRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.SetReviewVisibilityResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CatalogServiceMethodDescriptorSupplier("SetReviewVisibility"))
              .build();
        }
      }
    }
    return getSetReviewVisibilityMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.ReplyProductReviewRequest,
      com.ecommerce.contracts.catalog.v1.ReplyProductReviewResponse> getReplyProductReviewMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ReplyProductReview",
      requestType = com.ecommerce.contracts.catalog.v1.ReplyProductReviewRequest.class,
      responseType = com.ecommerce.contracts.catalog.v1.ReplyProductReviewResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.ReplyProductReviewRequest,
      com.ecommerce.contracts.catalog.v1.ReplyProductReviewResponse> getReplyProductReviewMethod() {
    io.grpc.MethodDescriptor<com.ecommerce.contracts.catalog.v1.ReplyProductReviewRequest, com.ecommerce.contracts.catalog.v1.ReplyProductReviewResponse> getReplyProductReviewMethod;
    if ((getReplyProductReviewMethod = CatalogServiceGrpc.getReplyProductReviewMethod) == null) {
      synchronized (CatalogServiceGrpc.class) {
        if ((getReplyProductReviewMethod = CatalogServiceGrpc.getReplyProductReviewMethod) == null) {
          CatalogServiceGrpc.getReplyProductReviewMethod = getReplyProductReviewMethod =
              io.grpc.MethodDescriptor.<com.ecommerce.contracts.catalog.v1.ReplyProductReviewRequest, com.ecommerce.contracts.catalog.v1.ReplyProductReviewResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ReplyProductReview"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.ReplyProductReviewRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.ecommerce.contracts.catalog.v1.ReplyProductReviewResponse.getDefaultInstance()))
              .setSchemaDescriptor(new CatalogServiceMethodDescriptorSupplier("ReplyProductReview"))
              .build();
        }
      }
    }
    return getReplyProductReviewMethod;
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

    /**
     */
    default void listAdminProducts(com.ecommerce.contracts.catalog.v1.ListAdminProductsRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.ListAdminProductsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListAdminProductsMethod(), responseObserver);
    }

    /**
     */
    default void listInventoryCatalogItems(com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListInventoryCatalogItemsMethod(), responseObserver);
    }

    /**
     */
    default void getAdminProduct(com.ecommerce.contracts.catalog.v1.GetAdminProductRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.GetAdminProductResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetAdminProductMethod(), responseObserver);
    }

    /**
     */
    default void createProduct(com.ecommerce.contracts.catalog.v1.CreateProductRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.CreateProductResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getCreateProductMethod(), responseObserver);
    }

    /**
     */
    default void updateProduct(com.ecommerce.contracts.catalog.v1.UpdateProductRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.UpdateProductResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getUpdateProductMethod(), responseObserver);
    }

    /**
     */
    default void createProductMediaUpload(com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getCreateProductMediaUploadMethod(), responseObserver);
    }

    /**
     */
    default void listProductReviews(com.ecommerce.contracts.catalog.v1.ListProductReviewsRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.ListProductReviewsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListProductReviewsMethod(), responseObserver);
    }

    /**
     */
    default void getReviewEligibility(com.ecommerce.contracts.catalog.v1.GetReviewEligibilityRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.GetReviewEligibilityResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetReviewEligibilityMethod(), responseObserver);
    }

    /**
     */
    default void createProductReview(com.ecommerce.contracts.catalog.v1.CreateProductReviewRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.CreateProductReviewResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getCreateProductReviewMethod(), responseObserver);
    }

    /**
     */
    default void updateProductReview(com.ecommerce.contracts.catalog.v1.UpdateProductReviewRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.UpdateProductReviewResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getUpdateProductReviewMethod(), responseObserver);
    }

    /**
     */
    default void deleteProductReview(com.ecommerce.contracts.catalog.v1.DeleteProductReviewRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.DeleteProductReviewResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getDeleteProductReviewMethod(), responseObserver);
    }

    /**
     */
    default void createReviewMediaUpload(com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getCreateReviewMediaUploadMethod(), responseObserver);
    }

    /**
     */
    default void listAdminReviews(com.ecommerce.contracts.catalog.v1.ListAdminReviewsRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.ListAdminReviewsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListAdminReviewsMethod(), responseObserver);
    }

    /**
     */
    default void setReviewVisibility(com.ecommerce.contracts.catalog.v1.SetReviewVisibilityRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.SetReviewVisibilityResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getSetReviewVisibilityMethod(), responseObserver);
    }

    /**
     */
    default void replyProductReview(com.ecommerce.contracts.catalog.v1.ReplyProductReviewRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.ReplyProductReviewResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getReplyProductReviewMethod(), responseObserver);
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

    /**
     */
    public void listAdminProducts(com.ecommerce.contracts.catalog.v1.ListAdminProductsRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.ListAdminProductsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListAdminProductsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void listInventoryCatalogItems(com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListInventoryCatalogItemsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void getAdminProduct(com.ecommerce.contracts.catalog.v1.GetAdminProductRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.GetAdminProductResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetAdminProductMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void createProduct(com.ecommerce.contracts.catalog.v1.CreateProductRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.CreateProductResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getCreateProductMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void updateProduct(com.ecommerce.contracts.catalog.v1.UpdateProductRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.UpdateProductResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getUpdateProductMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void createProductMediaUpload(com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getCreateProductMediaUploadMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void listProductReviews(com.ecommerce.contracts.catalog.v1.ListProductReviewsRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.ListProductReviewsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListProductReviewsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void getReviewEligibility(com.ecommerce.contracts.catalog.v1.GetReviewEligibilityRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.GetReviewEligibilityResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetReviewEligibilityMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void createProductReview(com.ecommerce.contracts.catalog.v1.CreateProductReviewRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.CreateProductReviewResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getCreateProductReviewMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void updateProductReview(com.ecommerce.contracts.catalog.v1.UpdateProductReviewRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.UpdateProductReviewResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getUpdateProductReviewMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void deleteProductReview(com.ecommerce.contracts.catalog.v1.DeleteProductReviewRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.DeleteProductReviewResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getDeleteProductReviewMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void createReviewMediaUpload(com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getCreateReviewMediaUploadMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void listAdminReviews(com.ecommerce.contracts.catalog.v1.ListAdminReviewsRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.ListAdminReviewsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListAdminReviewsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void setReviewVisibility(com.ecommerce.contracts.catalog.v1.SetReviewVisibilityRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.SetReviewVisibilityResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getSetReviewVisibilityMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void replyProductReview(com.ecommerce.contracts.catalog.v1.ReplyProductReviewRequest request,
        io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.ReplyProductReviewResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getReplyProductReviewMethod(), getCallOptions()), request, responseObserver);
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

    /**
     */
    public com.ecommerce.contracts.catalog.v1.ListAdminProductsResponse listAdminProducts(com.ecommerce.contracts.catalog.v1.ListAdminProductsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListAdminProductsMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsResponse listInventoryCatalogItems(com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListInventoryCatalogItemsMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.catalog.v1.GetAdminProductResponse getAdminProduct(com.ecommerce.contracts.catalog.v1.GetAdminProductRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetAdminProductMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.catalog.v1.CreateProductResponse createProduct(com.ecommerce.contracts.catalog.v1.CreateProductRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getCreateProductMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.catalog.v1.UpdateProductResponse updateProduct(com.ecommerce.contracts.catalog.v1.UpdateProductRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getUpdateProductMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadResponse createProductMediaUpload(com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getCreateProductMediaUploadMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.catalog.v1.ListProductReviewsResponse listProductReviews(com.ecommerce.contracts.catalog.v1.ListProductReviewsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListProductReviewsMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.catalog.v1.GetReviewEligibilityResponse getReviewEligibility(com.ecommerce.contracts.catalog.v1.GetReviewEligibilityRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetReviewEligibilityMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.catalog.v1.CreateProductReviewResponse createProductReview(com.ecommerce.contracts.catalog.v1.CreateProductReviewRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getCreateProductReviewMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.catalog.v1.UpdateProductReviewResponse updateProductReview(com.ecommerce.contracts.catalog.v1.UpdateProductReviewRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getUpdateProductReviewMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.catalog.v1.DeleteProductReviewResponse deleteProductReview(com.ecommerce.contracts.catalog.v1.DeleteProductReviewRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getDeleteProductReviewMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadResponse createReviewMediaUpload(com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getCreateReviewMediaUploadMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.catalog.v1.ListAdminReviewsResponse listAdminReviews(com.ecommerce.contracts.catalog.v1.ListAdminReviewsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListAdminReviewsMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.catalog.v1.SetReviewVisibilityResponse setReviewVisibility(com.ecommerce.contracts.catalog.v1.SetReviewVisibilityRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getSetReviewVisibilityMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.ecommerce.contracts.catalog.v1.ReplyProductReviewResponse replyProductReview(com.ecommerce.contracts.catalog.v1.ReplyProductReviewRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getReplyProductReviewMethod(), getCallOptions(), request);
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

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.catalog.v1.ListAdminProductsResponse> listAdminProducts(
        com.ecommerce.contracts.catalog.v1.ListAdminProductsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListAdminProductsMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsResponse> listInventoryCatalogItems(
        com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListInventoryCatalogItemsMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.catalog.v1.GetAdminProductResponse> getAdminProduct(
        com.ecommerce.contracts.catalog.v1.GetAdminProductRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetAdminProductMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.catalog.v1.CreateProductResponse> createProduct(
        com.ecommerce.contracts.catalog.v1.CreateProductRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getCreateProductMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.catalog.v1.UpdateProductResponse> updateProduct(
        com.ecommerce.contracts.catalog.v1.UpdateProductRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getUpdateProductMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadResponse> createProductMediaUpload(
        com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getCreateProductMediaUploadMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.catalog.v1.ListProductReviewsResponse> listProductReviews(
        com.ecommerce.contracts.catalog.v1.ListProductReviewsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListProductReviewsMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.catalog.v1.GetReviewEligibilityResponse> getReviewEligibility(
        com.ecommerce.contracts.catalog.v1.GetReviewEligibilityRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetReviewEligibilityMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.catalog.v1.CreateProductReviewResponse> createProductReview(
        com.ecommerce.contracts.catalog.v1.CreateProductReviewRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getCreateProductReviewMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.catalog.v1.UpdateProductReviewResponse> updateProductReview(
        com.ecommerce.contracts.catalog.v1.UpdateProductReviewRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getUpdateProductReviewMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.catalog.v1.DeleteProductReviewResponse> deleteProductReview(
        com.ecommerce.contracts.catalog.v1.DeleteProductReviewRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getDeleteProductReviewMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadResponse> createReviewMediaUpload(
        com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getCreateReviewMediaUploadMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.catalog.v1.ListAdminReviewsResponse> listAdminReviews(
        com.ecommerce.contracts.catalog.v1.ListAdminReviewsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListAdminReviewsMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.catalog.v1.SetReviewVisibilityResponse> setReviewVisibility(
        com.ecommerce.contracts.catalog.v1.SetReviewVisibilityRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getSetReviewVisibilityMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.ecommerce.contracts.catalog.v1.ReplyProductReviewResponse> replyProductReview(
        com.ecommerce.contracts.catalog.v1.ReplyProductReviewRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getReplyProductReviewMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_GET_PRODUCT = 0;
  private static final int METHODID_SEARCH_PRODUCTS = 1;
  private static final int METHODID_SOFT_DELETE_PRODUCT = 2;
  private static final int METHODID_LIST_ADMIN_PRODUCTS = 3;
  private static final int METHODID_LIST_INVENTORY_CATALOG_ITEMS = 4;
  private static final int METHODID_GET_ADMIN_PRODUCT = 5;
  private static final int METHODID_CREATE_PRODUCT = 6;
  private static final int METHODID_UPDATE_PRODUCT = 7;
  private static final int METHODID_CREATE_PRODUCT_MEDIA_UPLOAD = 8;
  private static final int METHODID_LIST_PRODUCT_REVIEWS = 9;
  private static final int METHODID_GET_REVIEW_ELIGIBILITY = 10;
  private static final int METHODID_CREATE_PRODUCT_REVIEW = 11;
  private static final int METHODID_UPDATE_PRODUCT_REVIEW = 12;
  private static final int METHODID_DELETE_PRODUCT_REVIEW = 13;
  private static final int METHODID_CREATE_REVIEW_MEDIA_UPLOAD = 14;
  private static final int METHODID_LIST_ADMIN_REVIEWS = 15;
  private static final int METHODID_SET_REVIEW_VISIBILITY = 16;
  private static final int METHODID_REPLY_PRODUCT_REVIEW = 17;

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
        case METHODID_LIST_ADMIN_PRODUCTS:
          serviceImpl.listAdminProducts((com.ecommerce.contracts.catalog.v1.ListAdminProductsRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.ListAdminProductsResponse>) responseObserver);
          break;
        case METHODID_LIST_INVENTORY_CATALOG_ITEMS:
          serviceImpl.listInventoryCatalogItems((com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsResponse>) responseObserver);
          break;
        case METHODID_GET_ADMIN_PRODUCT:
          serviceImpl.getAdminProduct((com.ecommerce.contracts.catalog.v1.GetAdminProductRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.GetAdminProductResponse>) responseObserver);
          break;
        case METHODID_CREATE_PRODUCT:
          serviceImpl.createProduct((com.ecommerce.contracts.catalog.v1.CreateProductRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.CreateProductResponse>) responseObserver);
          break;
        case METHODID_UPDATE_PRODUCT:
          serviceImpl.updateProduct((com.ecommerce.contracts.catalog.v1.UpdateProductRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.UpdateProductResponse>) responseObserver);
          break;
        case METHODID_CREATE_PRODUCT_MEDIA_UPLOAD:
          serviceImpl.createProductMediaUpload((com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadResponse>) responseObserver);
          break;
        case METHODID_LIST_PRODUCT_REVIEWS:
          serviceImpl.listProductReviews((com.ecommerce.contracts.catalog.v1.ListProductReviewsRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.ListProductReviewsResponse>) responseObserver);
          break;
        case METHODID_GET_REVIEW_ELIGIBILITY:
          serviceImpl.getReviewEligibility((com.ecommerce.contracts.catalog.v1.GetReviewEligibilityRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.GetReviewEligibilityResponse>) responseObserver);
          break;
        case METHODID_CREATE_PRODUCT_REVIEW:
          serviceImpl.createProductReview((com.ecommerce.contracts.catalog.v1.CreateProductReviewRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.CreateProductReviewResponse>) responseObserver);
          break;
        case METHODID_UPDATE_PRODUCT_REVIEW:
          serviceImpl.updateProductReview((com.ecommerce.contracts.catalog.v1.UpdateProductReviewRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.UpdateProductReviewResponse>) responseObserver);
          break;
        case METHODID_DELETE_PRODUCT_REVIEW:
          serviceImpl.deleteProductReview((com.ecommerce.contracts.catalog.v1.DeleteProductReviewRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.DeleteProductReviewResponse>) responseObserver);
          break;
        case METHODID_CREATE_REVIEW_MEDIA_UPLOAD:
          serviceImpl.createReviewMediaUpload((com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadResponse>) responseObserver);
          break;
        case METHODID_LIST_ADMIN_REVIEWS:
          serviceImpl.listAdminReviews((com.ecommerce.contracts.catalog.v1.ListAdminReviewsRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.ListAdminReviewsResponse>) responseObserver);
          break;
        case METHODID_SET_REVIEW_VISIBILITY:
          serviceImpl.setReviewVisibility((com.ecommerce.contracts.catalog.v1.SetReviewVisibilityRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.SetReviewVisibilityResponse>) responseObserver);
          break;
        case METHODID_REPLY_PRODUCT_REVIEW:
          serviceImpl.replyProductReview((com.ecommerce.contracts.catalog.v1.ReplyProductReviewRequest) request,
              (io.grpc.stub.StreamObserver<com.ecommerce.contracts.catalog.v1.ReplyProductReviewResponse>) responseObserver);
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
        .addMethod(
          getListAdminProductsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.catalog.v1.ListAdminProductsRequest,
              com.ecommerce.contracts.catalog.v1.ListAdminProductsResponse>(
                service, METHODID_LIST_ADMIN_PRODUCTS)))
        .addMethod(
          getListInventoryCatalogItemsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsRequest,
              com.ecommerce.contracts.catalog.v1.ListInventoryCatalogItemsResponse>(
                service, METHODID_LIST_INVENTORY_CATALOG_ITEMS)))
        .addMethod(
          getGetAdminProductMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.catalog.v1.GetAdminProductRequest,
              com.ecommerce.contracts.catalog.v1.GetAdminProductResponse>(
                service, METHODID_GET_ADMIN_PRODUCT)))
        .addMethod(
          getCreateProductMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.catalog.v1.CreateProductRequest,
              com.ecommerce.contracts.catalog.v1.CreateProductResponse>(
                service, METHODID_CREATE_PRODUCT)))
        .addMethod(
          getUpdateProductMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.catalog.v1.UpdateProductRequest,
              com.ecommerce.contracts.catalog.v1.UpdateProductResponse>(
                service, METHODID_UPDATE_PRODUCT)))
        .addMethod(
          getCreateProductMediaUploadMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadRequest,
              com.ecommerce.contracts.catalog.v1.CreateProductMediaUploadResponse>(
                service, METHODID_CREATE_PRODUCT_MEDIA_UPLOAD)))
        .addMethod(
          getListProductReviewsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.catalog.v1.ListProductReviewsRequest,
              com.ecommerce.contracts.catalog.v1.ListProductReviewsResponse>(
                service, METHODID_LIST_PRODUCT_REVIEWS)))
        .addMethod(
          getGetReviewEligibilityMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.catalog.v1.GetReviewEligibilityRequest,
              com.ecommerce.contracts.catalog.v1.GetReviewEligibilityResponse>(
                service, METHODID_GET_REVIEW_ELIGIBILITY)))
        .addMethod(
          getCreateProductReviewMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.catalog.v1.CreateProductReviewRequest,
              com.ecommerce.contracts.catalog.v1.CreateProductReviewResponse>(
                service, METHODID_CREATE_PRODUCT_REVIEW)))
        .addMethod(
          getUpdateProductReviewMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.catalog.v1.UpdateProductReviewRequest,
              com.ecommerce.contracts.catalog.v1.UpdateProductReviewResponse>(
                service, METHODID_UPDATE_PRODUCT_REVIEW)))
        .addMethod(
          getDeleteProductReviewMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.catalog.v1.DeleteProductReviewRequest,
              com.ecommerce.contracts.catalog.v1.DeleteProductReviewResponse>(
                service, METHODID_DELETE_PRODUCT_REVIEW)))
        .addMethod(
          getCreateReviewMediaUploadMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadRequest,
              com.ecommerce.contracts.catalog.v1.CreateReviewMediaUploadResponse>(
                service, METHODID_CREATE_REVIEW_MEDIA_UPLOAD)))
        .addMethod(
          getListAdminReviewsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.catalog.v1.ListAdminReviewsRequest,
              com.ecommerce.contracts.catalog.v1.ListAdminReviewsResponse>(
                service, METHODID_LIST_ADMIN_REVIEWS)))
        .addMethod(
          getSetReviewVisibilityMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.catalog.v1.SetReviewVisibilityRequest,
              com.ecommerce.contracts.catalog.v1.SetReviewVisibilityResponse>(
                service, METHODID_SET_REVIEW_VISIBILITY)))
        .addMethod(
          getReplyProductReviewMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.ecommerce.contracts.catalog.v1.ReplyProductReviewRequest,
              com.ecommerce.contracts.catalog.v1.ReplyProductReviewResponse>(
                service, METHODID_REPLY_PRODUCT_REVIEW)))
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
              .addMethod(getListAdminProductsMethod())
              .addMethod(getListInventoryCatalogItemsMethod())
              .addMethod(getGetAdminProductMethod())
              .addMethod(getCreateProductMethod())
              .addMethod(getUpdateProductMethod())
              .addMethod(getCreateProductMediaUploadMethod())
              .addMethod(getListProductReviewsMethod())
              .addMethod(getGetReviewEligibilityMethod())
              .addMethod(getCreateProductReviewMethod())
              .addMethod(getUpdateProductReviewMethod())
              .addMethod(getDeleteProductReviewMethod())
              .addMethod(getCreateReviewMediaUploadMethod())
              .addMethod(getListAdminReviewsMethod())
              .addMethod(getSetReviewVisibilityMethod())
              .addMethod(getReplyProductReviewMethod())
              .build();
        }
      }
    }
    return result;
  }
}
