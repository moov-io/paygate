# Addenda05

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Id** | **string** | Client defined string used as a reference to this record. | [optional] 
**TypeCode** | **string** | 05 - NACHA regulations | [optional] 
**PaymentRelatedInformation** | **string** | Text for describing the related payment | [optional] 
**SequenceNumber** | **float32** | SequenceNumber is consecutively assigned to each Addenda05 Record following an Entry Detail Record. The first addenda05 sequence number must always be a 1. | [optional] 
**EntryDetailSequenceNumber** | **float32** | EntryDetailSequenceNumber contains the ascending sequence number section of the Entry Detail or Corporate Entry Detail Record&#39;s trace number This number is the same as the last seven digits of the trace number of the related Entry Detail Record or Corporate Entry Detail Record.  | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


