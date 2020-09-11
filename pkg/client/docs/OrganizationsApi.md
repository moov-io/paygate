# \OrganizationsApi

All URIs are relative to *http://localhost:8082*

Method | HTTP request | Description
------------- | ------------- | -------------
[**CreateOrganization**](OrganizationsApi.md#CreateOrganization) | **Post** /organizations | Create Organization
[**GetOrganizations**](OrganizationsApi.md#GetOrganizations) | **Get** /organizations | Get Organizations
[**UpdateOrganization**](OrganizationsApi.md#UpdateOrganization) | **Put** /organizations/{organizationID} | Update Organization



## CreateOrganization

> Organization CreateOrganization(ctx, xTenant, createOrganization, optional)

Create Organization

Create a new Organization under PayGate

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**xTenant** | **string**| Unique tenantID set by an auth proxy or client to identify and isolate objects. | 
**createOrganization** | [**CreateOrganization**](CreateOrganization.md)|  | 
 **optional** | ***CreateOrganizationOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a CreateOrganizationOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **xRequestID** | **optional.String**| Optional requestID allows application developer to trace requests through the systems logs | 

### Return type

[**Organization**](Organization.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetOrganizations

> []Organization GetOrganizations(ctx, xTenant, optional)

Get Organizations

Retrieve all Organizations for the given tenantID

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**xTenant** | **string**| Unique tenantID set by an auth proxy or client to identify and isolate objects. | 
 **optional** | ***GetOrganizationsOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetOrganizationsOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **xRequestID** | **optional.String**| Optional requestID allows application developer to trace requests through the systems logs | 

### Return type

[**[]Organization**](Organization.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## UpdateOrganization

> Organization UpdateOrganization(ctx, organizationID, xTenant, createOrganization, optional)

Update Organization

Update metadata for an Organization

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**organizationID** | **string**| organizationID for the Organization to update | 
**xTenant** | **string**| Unique tenantID set by an auth proxy or client to identify and isolate objects. | 
**createOrganization** | [**CreateOrganization**](CreateOrganization.md)|  | 
 **optional** | ***UpdateOrganizationOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a UpdateOrganizationOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **xRequestID** | **optional.String**| Optional requestID allows application developer to trace requests through the systems logs | 

### Return type

[**Organization**](Organization.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

