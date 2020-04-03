# Addenda18

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Id** | **string** | Client defined string used as a reference to this record. | [optional] 
**TypeCode** | **string** | 10 - NACHA regulations | [optional] 
**ForeignCorrespondentBankName** | **string** | Name of the Foreign Correspondent Bank | [optional] 
**ForeignCorrespondentBankIDNumberQualifier** | **string** | Foreign Correspondent Bank Identification Number Qualifier contains a 2-digit code that identifies the numbering scheme used in the Foreign Correspondent Bank Identification Number field. Code values for this field are   \&quot;01\&quot; &#x3D; National Clearing System   \&quot;02\&quot; &#x3D; BIC Code   \&quot;03\&quot; &#x3D; IBAN Code  | [optional] 
**ForeignCorrespondentBankIDNumber** | **string** | Foreign Correspondent Bank Identification Number contains the bank ID number of the Foreign Correspondent Bank | [optional] 
**ForeignCorrespondentBankBranchCountryCode** | **string** | Foreign Correspondent Bank Branch Country Code contains the two-character code, as approved by the International Organization for Standardization (ISO), to identify the country in which the branch of the Foreign Correspondent Bank is located. Values can be found on the International Organization for Standardization website: www.iso.org  | [optional] 
**SequenceNumber** | **float32** | SequenceNumber is consecutively assigned to each Addenda17 Record following an Entry Detail Record. The first addenda17 sequence number must always be a 1.  | [optional] 
**EntryDetailSequenceNumber** | **float32** | EntryDetailSequenceNumber contains the ascending sequence number section of the Entry Detail or Corporate Entry Detail Record&#39;s trace number This number is the same as the last seven digits of the trace number of the related Entry Detail Record or Corporate Entry Detail Record.  | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


