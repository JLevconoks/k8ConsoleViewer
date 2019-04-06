package main

import (
	"testing"
)

func TestProcessPodResponse(t *testing.T) {
	tt := []struct {
		name        string
		namespace   string
		input       []byte
		podCount    int
		expectError bool
	}{
		{name: "Single Pod Info",
			namespace: "testNamespace",
			input: []byte("NAME                                         READY     STATUS    RESTARTS   AGE\n" +
				"sample-pod-55894cf7cb-tb999       1/1       Running   0          6h"),
			podCount:    1,
			expectError: false,
		},
		{name: "Mupltiple Pod Info",
			namespace: "testNamespace",
			input: []byte("NAME                                         READY     STATUS    RESTARTS   AGE\n" +
				"sample-pod-55894cf7cb-tb999       1/1       Running   0          6h\n" +
				"sample-pod-55894cf7cb-tb888       1/1       Running   0          6h\n" +
				"sample-pod-55894cf7cb-tb666       1/1       Running   0          6h"),
			podCount:    3,
			expectError: false,
		},
		{name: "Wrong Header Line",
			namespace: "testNamespace",
			input: []byte("AME                                         READY     STATUS    RESTARTS   AGE\n" +
				"sample-pod-55894cf7cb-tb666       1/1       Running   0          6h"),
			podCount:    0,
			expectError: true,
		},
		{name: "Wrong Pod line",
			namespace: "testNamespace",
			input: []byte("NAME                                         READY     STATUS    RESTARTS   AGE\n" +
				"sample-pod-55894cf7cb-tb666       A/1       Running   0          6h"),
			podCount:    0,
			expectError: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ns, err := processPodResponse(tc.namespace, tc.input)

			if tc.expectError && err == nil {
				t.Error("Expected error but not received")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Error not expected, got: %v", err)
			}
			if err == nil {
				if ns.Name != tc.namespace {
					t.Errorf("Expected namespace name '%v', got '%v'", tc.namespace, ns.Name)
				}
				if len(ns.Pods) != tc.podCount {
					t.Errorf("Expected '%v' pod, got '%v'", tc.podCount, len(ns.Pods))
				}
			}
		})
	}
}

func TestParsePodLine(t *testing.T) {
	tt := []struct {
		name        string
		input       string
		expected    Pod
		expectError bool
	}{
		{
			name:  "Valid Input",
			input: "sample-pod-55894cf7cb-tb999       1/1       Running   0          6h",
			expected: Pod{
				Name:     "sample-pod-55894cf7cb-tb999",
				Total:    1,
				Ready:    1,
				Status:   "Running",
				Restarts: "0",
				Age:      "6h",
			},
			expectError: false,
		},
		{
			name:        "Input missing fields",
			input:       "sample-pod-55894cf7cb-tb999        Running   0          6h",
			expectError: true,
		},
		{
			name:        "Input is invalid",
			input:       "sample-pod-55894cf7cb-tb999       A/B       Running   0          6h",
			expectError: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parsePodLine(tc.input)
			pod := tc.expected
			if tc.expectError && err == nil {
				t.Error("Expected error but not received")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Error not expected, got: %v", err)
			}
			if err == nil {
				if result.Name != pod.Name {
					t.Errorf("Expected pod Name '%v', got '%v'", pod.Name, result.Name)
				}
				if result.Total != pod.Total {
					t.Errorf("Expected pod Total '%v', got '%v'", pod.Total, result.Total)
				}
				if result.Ready != pod.Ready {
					t.Errorf("Expected pod Ready '%v', got '%v'", pod.Ready, result.Ready)
				}
				if result.Status != pod.Status {
					t.Errorf("Expected pod Status '%v', got '%v'", pod.Status, result.Status)
				}
				if result.Restarts != pod.Restarts {
					t.Errorf("Expected pod name '%v', got '%v'", pod.Restarts, result.Restarts)
				}
				if result.Age != pod.Age {
					t.Errorf("Expected pod name '%v', got '%v'", pod.Age, result.Age)
				}
			}

		})
	}
}

func TestCleanSplit(t *testing.T) {
	input := "sample-pod-55894cf7cb-tb999       1/1       Running   0          6h"
	expected := []string{
		"sample-pod-55894cf7cb-tb999",
		"1/1",
		"Running",
		"0",
		"6h",
	}

	result := cleanSplit(input)

	for k, v := range result {
		if v != expected[k] {
			t.Errorf("Expected '%v' for index '%v', got '%v'", expected[k], k, v)
		}
	}
}

func TestSplitReadyString(t *testing.T) {
	tt := []struct {
		name          string
		input         string
		expectedReady int
		expectedTotal int
		expectError   bool
	}{
		{name: "1/1", input: "1/1", expectedReady: 1, expectedTotal: 1, expectError: false},
		{name: "2/10", input: "2/10", expectedReady: 2, expectedTotal: 10, expectError: false},
		{name: "ABC/1", input: "ABC/1", expectedReady: 0, expectedTotal: 0, expectError: true},
		{name: "1/ABC", input: "1/ABC", expectedReady: 1, expectedTotal: 0, expectError: true},
		{name: "ABC", input: "ABC", expectedReady: 0, expectedTotal: 0, expectError: true},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ready, total, err := splitReadyString(tc.input)
			if tc.expectError && err == nil {
				t.Error("Expected error but not received")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Error not expected, got: %v", err)
			}
			if ready != tc.expectedReady {
				t.Errorf("Expected Ready '%v', got '%v'", tc.expectedReady, ready)
			}
			if total != tc.expectedTotal {
				t.Errorf("Expected Ready '%v', got '%v'", tc.expectedTotal, total)
			}
		})
	}
}
