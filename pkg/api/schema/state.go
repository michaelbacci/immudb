/*
Copyright 2019-2020 vChain, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package schema

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/codenotary/immudb/pkg/signer"
)

func (state *ImmutableState) ToBytes() []byte {
	b := make([]byte, 8+sha256.Size)
	binary.BigEndian.PutUint64(b[:], state.TxId)
	copy(b[8:], state.TxHash[:])
	return b
}

//CheckSignature
func (state *ImmutableState) CheckSignature() (ok bool, err error) {
	if state.Signature == nil {
		return false, errors.New("no signature found")
	}
	if state.Signature.PublicKey == nil {
		return false, errors.New("no public key found")
	}

	return signer.Verify(state.ToBytes(), state.Signature.Signature, state.Signature.PublicKey)
}