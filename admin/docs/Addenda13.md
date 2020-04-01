# Addenda13

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Id** | **string** | Client defined string used as a reference to this record. | [optional] 
**TypeCode** | **string** | 10 - NACHA regulations | [optional] 
**ODFIName** | **string** | Originating DFI Name For Outbound IAT Entries, this field must contain the name of the U.S. ODFI. For Inbound IATs: Name of the foreign bank providing funding for the payment transaction  | [optional] 
**ODFIIDNumberQualifier** | **string** | Originating DFI Identification Number Qualifier For Inbound IATs: The 2-digit code that identifies the numbering scheme used in the Foreign DFI Identification Number field 01 &#x3D; National Clearing System 02 &#x3D; BIC Code 03 &#x3D; IBAN Code  | [optional] 
**ODFIBranchCountryCode** | **string** | Originating DFI Branch Country Code USb &#x3D; United States //(\&quot;b\&quot; indicates a blank space) For Inbound IATs: This 3 position field contains a 2-character code as approved by the International Organization for Standardization (ISO) used to identify the country in which the branch of the bank that originated the entry is located. Values for other countries can be found on the International Organization for Standardization website: www.iso.org.  | [optional] 
**EntryDetailSequenceNumber** | **float32** | EntryDetailSequenceNumber contains the ascending sequence number section of the Entry Detail or Corporate Entry Detail Record&#39;s trace number This number is the same as the last seven digits of the trace number of the related Entry Detail Record or Corporate Entry Detail Record.  | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


