/*
Copyright 2018 The Kubernetes Authors.

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

package cmd_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/renstrom/dedent"

	kubeadmapiv1alpha3 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1alpha3"
	"k8s.io/kubernetes/cmd/kubeadm/app/cmd"
	"k8s.io/kubernetes/cmd/kubeadm/app/features"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/config"
	utilruntime "k8s.io/kubernetes/cmd/kubeadm/app/util/runtime"
	"k8s.io/utils/exec"
	fakeexec "k8s.io/utils/exec/testing"
)

const (
	defaultNumberOfImages = 8
	// dummyKubernetesVersion is just used for unit testing, in order to not make
	// kubeadm lookup dl.k8s.io to resolve what the latest stable release is
	dummyKubernetesVersion = "v1.10.0"
)

func TestNewCmdConfigImagesList(t *testing.T) {
	var output bytes.Buffer
	mockK8sVersion := dummyKubernetesVersion
	images := cmd.NewCmdConfigImagesList(&output, &mockK8sVersion)
	images.Run(nil, nil)
	actual := strings.Split(output.String(), "\n")
	if len(actual) != defaultNumberOfImages {
		t.Fatalf("Expected %v but found %v images", defaultNumberOfImages, len(actual))
	}
}

func TestImagesListRunWithCustomConfigPath(t *testing.T) {
	testcases := []struct {
		name               string
		expectedImageCount int
		// each string provided here must appear in at least one image returned by Run
		expectedImageSubstrings []string
		configContents          []byte
	}{
		{
			name:               "set k8s version",
			expectedImageCount: defaultNumberOfImages,
			expectedImageSubstrings: []string{
				":v1.10.1",
			},
			configContents: []byte(dedent.Dedent(`
				apiVersion: kubeadm.k8s.io/v1alpha3
				kind: InitConfiguration
				kubernetesVersion: v1.10.1
			`)),
		},
		{
			name:               "use coredns",
			expectedImageCount: defaultNumberOfImages,
			expectedImageSubstrings: []string{
				"coredns",
			},
			configContents: []byte(dedent.Dedent(`
				apiVersion: kubeadm.k8s.io/v1alpha3
				kind: InitConfiguration
				kubernetesVersion: v1.11.0
				featureGates:
				  CoreDNS: True
			`)),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, err := ioutil.TempDir("", "kubeadm-images-test")
			if err != nil {
				t.Fatalf("Unable to create temporary directory: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			configFilePath := filepath.Join(tmpDir, "test-config-file")
			err = ioutil.WriteFile(configFilePath, tc.configContents, 0644)
			if err != nil {
				t.Fatalf("Failed writing a config file: %v", err)
			}

			i, err := cmd.NewImagesList(configFilePath, &kubeadmapiv1alpha3.InitConfiguration{
				KubernetesVersion: dummyKubernetesVersion,
			})
			if err != nil {
				t.Fatalf("Failed getting the kubeadm images command: %v", err)
			}
			var output bytes.Buffer
			if i.Run(&output) != nil {
				t.Fatalf("Error from running the images command: %v", err)
			}
			actual := strings.Split(output.String(), "\n")
			if len(actual) != tc.expectedImageCount {
				t.Fatalf("did not get the same number of images: actual: %v expected: %v. Actual value: %v", len(actual), tc.expectedImageCount, actual)
			}

			for _, substring := range tc.expectedImageSubstrings {
				if !strings.Contains(output.String(), substring) {
					t.Errorf("Expected to find %v but did not in this list of images: %v", substring, actual)
				}
			}
		})
	}
}

func TestConfigImagesListRunWithoutPath(t *testing.T) {
	testcases := []struct {
		name           string
		cfg            kubeadmapiv1alpha3.InitConfiguration
		expectedImages int
	}{
		{
			name:           "empty config",
			expectedImages: defaultNumberOfImages,
			cfg: kubeadmapiv1alpha3.InitConfiguration{
				KubernetesVersion: dummyKubernetesVersion,
			},
		},
		{
			name: "external etcd configuration",
			cfg: kubeadmapiv1alpha3.InitConfiguration{
				Etcd: kubeadmapiv1alpha3.Etcd{
					External: &kubeadmapiv1alpha3.ExternalEtcd{
						Endpoints: []string{"https://some.etcd.com:2379"},
					},
				},
				KubernetesVersion: dummyKubernetesVersion,
			},
			expectedImages: defaultNumberOfImages - 1,
		},
		{
			name: "coredns enabled",
			cfg: kubeadmapiv1alpha3.InitConfiguration{
				FeatureGates: map[string]bool{
					features.CoreDNS: true,
				},
				KubernetesVersion: dummyKubernetesVersion,
			},
			expectedImages: defaultNumberOfImages,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			i, err := cmd.NewImagesList("", &tc.cfg)
			if err != nil {
				t.Fatalf("did not expect an error while creating the Images command: %v", err)
			}

			var output bytes.Buffer
			if i.Run(&output) != nil {
				t.Fatalf("did not expect an error running the Images command: %v", err)
			}

			actual := strings.Split(output.String(), "\n")
			if len(actual) != tc.expectedImages {
				t.Fatalf("expected %v images but got %v", tc.expectedImages, actual)
			}
		})
	}
}

func TestImagesPull(t *testing.T) {
	fcmd := fakeexec.FakeCmd{
		CombinedOutputScript: []fakeexec.FakeCombinedOutputAction{
			func() ([]byte, error) { return nil, nil },
			func() ([]byte, error) { return nil, nil },
			func() ([]byte, error) { return nil, nil },
			func() ([]byte, error) { return nil, nil },
			func() ([]byte, error) { return nil, nil },
		},
	}

	fexec := fakeexec.FakeExec{
		CommandScript: []fakeexec.FakeCommandAction{
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
		},
		LookPathFunc: func(cmd string) (string, error) { return "/usr/bin/docker", nil },
	}

	containerRuntime, err := utilruntime.NewContainerRuntime(&fexec, kubeadmapiv1alpha3.DefaultCRISocket)
	if err != nil {
		t.Errorf("unexpected NewContainerRuntime error: %v", err)
	}

	images := []string{"a", "b", "c", "d", "a"}
	ip := cmd.NewImagesPull(containerRuntime, images)

	err = ip.PullAll()
	if err != nil {
		t.Fatalf("expected nil but found %v", err)
	}

	if fcmd.CombinedOutputCalls != len(images) {
		t.Errorf("expected %d calls, got %d", len(images), fcmd.CombinedOutputCalls)
	}
}

func TestMigrate(t *testing.T) {
	cfg := []byte(dedent.Dedent(`
		apiVersion: kubeadm.k8s.io/v1alpha3
		kind: InitConfiguration
		kubernetesVersion: v1.10.0
	`))
	configFile, cleanup := tempConfig(t, cfg)
	defer cleanup()

	var output bytes.Buffer
	command := cmd.NewCmdConfigMigrate(&output)
	err := command.Flags().Set("old-config", configFile)
	if err != nil {
		t.Fatalf("failed to set old-config flag")
	}
	command.Run(nil, nil)
	_, err = config.BytesToInternalConfig(output.Bytes())
	if err != nil {
		t.Fatalf("Could not read output back into internal type: %v", err)
	}
}

// Returns the name of the file created and a cleanup callback
func tempConfig(t *testing.T, config []byte) (string, func()) {
	t.Helper()
	tmpDir, err := ioutil.TempDir("", "kubeadm-migration-test")
	if err != nil {
		t.Fatalf("Unable to create temporary directory: %v", err)
	}
	configFilePath := filepath.Join(tmpDir, "test-config-file")
	err = ioutil.WriteFile(configFilePath, config, 0644)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed writing a config file: %v", err)
	}
	return configFilePath, func() {
		os.RemoveAll(tmpDir)
	}
}
