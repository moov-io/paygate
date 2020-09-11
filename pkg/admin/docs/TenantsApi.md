# \TenantsApi

All URIs are relative to *http://localhost:9092*

Method | HTTP request | Description
------------- | ------------- | -------------
[**CreateTenant**](TenantsApi.md#CreateTenant) | **Post** /tenants | Create Tenant



## CreateTenant

> Tenant CreateTenant(ctx, xTenant, createTenant, optional)

Create Tenant

Create a new Tenant under PayGate

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**xTenant** | **string**| Unique tenantID set by an auth proxy or client to identify and isolate objects. | 
**createTenant** | [**CreateTenant**](CreateTenant.md)|  | 
 **optional** | ***CreateTenantOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a CreateTenantOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **xRequestID** | **optional.String**| Optional requestID allows application developer to trace requests through the systems logs | 

### Return type

[**Tenant**](Tenant.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

