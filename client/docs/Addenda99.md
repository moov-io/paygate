# Addenda99

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Id** | **string** | Client defined string used as a reference to this record. | [optional] 
**TypeCode** | **string** | 99 - NACHA regulations | [optional] 
**ReturnCode** | **string** | Standard code used by an ACH Operator or RDFI to describe the reason for returning an Entry. | [optional] 
**OriginalTrace** | **string** | OriginalTrace This field contains the Trace Number as originally included on the forward Entry or Prenotification. The RDFI must include the Original Entry Trace Number in the Addenda Record of an Entry being returned to an ODFI, in the Addenda Record of an 98, within an Acknowledgment Entry, or with an RDFI request for a copy of an authorization.  | [optional] 
**DateOfDeath** | **string** | DateOfDeath The field date of death is to be supplied on Entries being returned for reason of death (return reason codes R14 and R15). Format YYMMDD (Y&#x3D;Year, M&#x3D;Month, D&#x3D;Day) | [optional] 
**OriginalDFI** | **string** | OriginalDFI field contains the Receiving DFI Identification (addenda.RDFIIdentification) as originally included on the forward Entry or Prenotification that the RDFI is returning or correcting. | [optional] 
**AddendaInformation** | **string** | Information related to the return | [optional] 
**TraceNumber** | **string** | Matches the Entry Detail Trace Number of the entry being returned. | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


