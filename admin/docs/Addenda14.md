# Addenda14

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Id** | **string** | Client defined string used as a reference to this record. | [optional] 
**TypeCode** | **string** | 10 - NACHA regulations | [optional] 
**RDFIName** | **string** | Name of the Receiver bank | [optional] 
**RDFIIDNumberQualifier** | **string** | Receiving DFI Identification Number Qualifier The 2-digit code that identifies the numbering scheme used in the Receiving DFI Identification Number field 01 &#x3D; National Clearing System 02 &#x3D; BIC Code 03 &#x3D; IBAN Code  | [optional] 
**RDFIIdentification** | **string** | This field contains the bank identification number of the DFI at which the Receiver maintains his account. | [optional] 
**RDFIBranchCountryCode** | **string** | Receiving DFI Branch Country Code USb\&quot; &#x3D; United States (\&quot;b\&quot; indicates a blank space) This 3 position field contains a 2-character code as approved by the International Organization for Standardization (ISO) used to identify the country in which the branch of the bank that receives the entry is located. Values for other countries can be found on the International Organization for Standardization website: www.iso.org  | [optional] 
**EntryDetailSequenceNumber** | **float32** | EntryDetailSequenceNumber contains the ascending sequence number section of the Entry Detail or Corporate Entry Detail Record&#39;s trace number This number is the same as the last seven digits of the trace number of the related Entry Detail Record or Corporate Entry Detail Record.  | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


