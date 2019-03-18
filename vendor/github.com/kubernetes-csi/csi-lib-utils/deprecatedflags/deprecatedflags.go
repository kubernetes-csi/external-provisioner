/*
Copyright 2019 The Kubernetes Authors.

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

// Package deprecatedflags can be used to declare a flag that is no
// longer supported.
package deprecatedflags

import (
	"flag"

	"k8s.io/klog"
)

// Add defines a deprecated option which used to take some kind of
// value. The type of the argument no longer matters, it gets ignored
// and instead using the option triggers a deprecation warning. The
// return code can be safely ignored and is only provided to support
// replacing functions like flag.Int in a global variable section.
func Add(name string) bool {
	flag.Var(deprecated{name: name}, name, "This option is deprecated.")
	return true
}

// AddBool defines a deprecated boolean option. Otherwise it behaves
// like Add.
func AddBool(name string) bool {
	flag.Var(deprecated{name: name, isBool: true}, name, "This option is deprecated.")
	return true
}

type deprecated struct {
	name   string
	isBool bool
}

var _ flag.Value = deprecated{}

func (d deprecated) String() string { return "" }
func (d deprecated) Set(value string) error {
	klog.Warningf("Warning: option %s=%q is deprecated and has no effect", d.name, value)
	return nil
}
func (d deprecated) Type() string     { return "" }
func (d deprecated) IsBoolFlag() bool { return d.isBool }
