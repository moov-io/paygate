The design of PayGate has certain deliberate choices made. These are made with our teams previous experience and current business requirements.

### Amount

PayGate treats amounts as strings (e.g. `USD 12.55`) with a static currency and the numeric portion internally as an integer. This allows us to transmit values between languages and systems without much worry in floating point conversions.
