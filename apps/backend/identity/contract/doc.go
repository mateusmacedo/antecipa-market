// Package contract is the published contract of identity: the Protobuf source
// in proto/ becomes gen/go only through buf generate, pinned by the DMPF plugin,
// and another context imports those packages, never the module of the app.
package contract
