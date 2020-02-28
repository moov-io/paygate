# Addenda98

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Id** | **string** | Client defined string used as a reference to this record. | [optional] 
**TypeCode** | **string** | 98 - NACHA regulations | [optional] 
**ChangeCode** | **string** | ChangeCode field contains a standard code used by an ACH Operator or RDFI to describe the reason for a change Entry. | [optional] 
**OriginalTrace** | **string** | OriginalTrace This field contains the Trace Number as originally included on the forward Entry or Prenotification. The RDFI must include the Original Entry Trace Number in the Addenda Record of an Entry being returned to an ODFI, in the Addenda Record of an 98, within an Acknowledgment Entry, or with an RDFI request for a copy of an authorization.  | [optional] 
**OriginalDFI** | **string** | The Receiving DFI Identification (addenda.RDFIIdentification) as originally included on the forward Entry or Prenotification that the RDFI is returning or correcting. | [optional] 
**CorrectedData** | **string** | Correct field value of what ChangeCode references | [optional] 
**TraceNumber** | **string** | Entry Detail Trace Number | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


