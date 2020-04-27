# \AdminApi

All URIs are relative to *http://localhost:9092*

Method | HTTP request | Description
------------- | ------------- | -------------
[**GetLivenessProbes**](AdminApi.md#GetLivenessProbes) | **Get** /live | Get Liveness Probes
[**GetVersion**](AdminApi.md#GetVersion) | **Get** /version | Get Version



## GetLivenessProbes

> LivenessProbes GetLivenessProbes(ctx, )

Get Liveness Probes

Get the status of each depdendency

### Required Parameters

This endpoint does not need any parameter.

### Return type

[**LivenessProbes**](LivenessProbes.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetVersion

> string GetVersion(ctx, )

Get Version

Show the current version of PayGate

### Required Parameters

This endpoint does not need any parameter.

### Return type

**string**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: text/plain

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

