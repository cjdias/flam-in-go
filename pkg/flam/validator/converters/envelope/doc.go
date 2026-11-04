// Package envelope is the validation error converter driver of the REST
// responses: its converters turn the
// [github.com/cjdias/flam-in-go/pkg/flam.ValidationError] values of a
// value failing its validation into a
// [github.com/cjdias/flam-in-go/pkg/flam/envelope.Envelope] holding one
// error per failure, of the status of the converter, 400 unless
// configured, identified by the id of the param and the code of the rule,
// with the message of the failure and the name of the param, at
// [ContextFieldParam], in its context, such as:
//
//	flam:
//	  validator:
//	    defaults:
//	      validators:
//	        converter: envelope
//
// making the validators answer the envelopes, which a handler writes with
// [github.com/cjdias/flam-in-go/pkg/flam/rest.WriteEnvelope], as
// [github.com/cjdias/flam-in-go/pkg/flam/rest.Validate] does. [Convert]
// converts the failures returned by a validator without converter.
package envelope
