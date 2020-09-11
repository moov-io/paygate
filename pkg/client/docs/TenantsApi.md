# \TenantsApi

All URIs are relative to *http://localhost:8082*

Method | HTTP request | Description
------------- | ------------- | -------------
[**UpdateTenant**](TenantsApi.md#UpdateTenant) | **Put** /tenants/{tenantID} | Update Tenant



## UpdateTenant

> UpdateTenant(ctx, tenantID, xTenant, updateTenant, optional)

Update Tenant

Update information for a Tenant

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**tenantID** | **string**| tenantID to identify which Tenant to update | 
**xTenant** | **string**| Unique tenantID set by an auth proxy or client to identify and isolate objects. | 
**updateTenant** | [**UpdateTenant**](UpdateTenant.md)|  | 
 **optional** | ***UpdateTenantOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a UpdateTenantOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **xRequestID** | **optional.String**| Optional requestID allows application developer to trace requests through the systems logs | 

### Return type

 (empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

