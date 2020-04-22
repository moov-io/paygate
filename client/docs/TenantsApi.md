# \TenantsApi

All URIs are relative to *http://localhost:8082*

Method | HTTP request | Description
------------- | ------------- | -------------
[**GetTenants**](TenantsApi.md#GetTenants) | **Get** /tenants | Get Tenants
[**UpdateTenant**](TenantsApi.md#UpdateTenant) | **Put** /tenants/{tenantID} | Update Tenant



## GetTenants

> []Tenant GetTenants(ctx, xUserID, optional)

Get Tenants

Retrieve all Tenants for the given userID

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**xUserID** | **string**| Unique userID set by an auth proxy or client to identify and isolate objects. | 
 **optional** | ***GetTenantsOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetTenantsOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **xRequestID** | **optional.String**| Optional requestID allows application developer to trace requests through the systems logs | 

### Return type

[**[]Tenant**](Tenant.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## UpdateTenant

> UpdateTenant(ctx, tenantID, xUserID, updateTenant, optional)

Update Tenant

Update information for a Tenant

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**tenantID** | **string**| tenantID to identify which Tenant to update | 
**xUserID** | **string**| Unique userID set by an auth proxy or client to identify and isolate objects. | 
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

