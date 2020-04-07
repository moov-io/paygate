# \GatewaysApi

All URIs are relative to *http://localhost:8082*

Method | HTTP request | Description
------------- | ------------- | -------------
[**AddGateway**](GatewaysApi.md#AddGateway) | **Post** /gateways | Update Gateway
[**GetGateways**](GatewaysApi.md#GetGateways) | **Get** /gateways | Gets Gatway



## AddGateway

> Gateway AddGateway(ctx, xUserID, createGateway, optional)

Update Gateway

Set the Gateway information for the specified user

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**xUserID** | **string**| Moov User ID | 
**createGateway** | [**CreateGateway**](CreateGateway.md)|  | 
 **optional** | ***AddGatewayOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a AddGatewayOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **xIdempotencyKey** | **optional.String**| Idempotent key in the header which expires after 24 hours. These strings should contain enough entropy for to not collide with each other in your requests. | 
 **xRequestID** | **optional.String**| Optional Request ID allows application developer to trace requests through the systems logs | 

### Return type

[**Gateway**](Gateway.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetGateways

> []Gateway GetGateways(ctx, xUserID, optional)

Gets Gatway

Get the Gateway information for the specified user

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**xUserID** | **string**| Moov User ID | 
 **optional** | ***GetGatewaysOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetGatewaysOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **xRequestID** | **optional.String**| Optional Request ID allows application developer to trace requests through the systems logs | 

### Return type

[**[]Gateway**](Gateway.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

