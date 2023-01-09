/*
Copyright 2019 Google LLC.

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

package v1beta1

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/spotify/flink-on-k8s-operator/internal/util"
	"k8s.io/apimachinery/pkg/api/resource"

	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const MaxStateAgeToRestore = int32(60)

var DefaultResources = corev1.ResourceRequirements{
	Requests: corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("200m"),
		corev1.ResourceMemory: resource.MustParse("512Mi"),
	},
	Limits: corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("2"),
		corev1.ResourceMemory: resource.MustParse("2Gi"),
	},
}

func TestValidateCreate(t *testing.T) {
	var jmReplicas int32 = DefaultJobManagerReplicas
	var tmReplicas int32 = DefaultTaskManagerReplicas
	var rpcPort int32 = 8001
	var blobPort int32 = 8002
	var queryPort int32 = 8003
	var uiPort int32 = 8004
	var dataPort int32 = 8005
	var jarFile = "gs://my-bucket/myjob.jar"
	var parallelism int32 = 2
	var maxStateAgeToRestoreSeconds = int32(60)
	var restartPolicy = JobRestartPolicyFromSavepointOnFailure
	var memoryProcessRatio int32 = 25
	var jobMode = JobModeDetached
	var resources = DefaultResources
	var cluster = FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: "default",
		},
		Spec: FlinkClusterSpec{
			FlinkVersion: "1.12",
			Image: ImageSpec{
				Name:       "flink:1.12.1",
				PullPolicy: corev1.PullPolicy("Always"),
			},
			JobManager: &JobManagerSpec{
				Replicas:    &jmReplicas,
				AccessScope: AccessScopeVPC,
				Ports: JobManagerPorts{
					RPC:   &rpcPort,
					Blob:  &blobPort,
					Query: &queryPort,
					UI:    &uiPort,
				},
				MemoryProcessRatio: &memoryProcessRatio,
				Resources:          resources,
			},
			TaskManager: &TaskManagerSpec{
				Replicas: &tmReplicas,
				Ports: TaskManagerPorts{
					RPC:   &rpcPort,
					Data:  &dataPort,
					Query: &queryPort,
				},
				MemoryProcessRatio: &memoryProcessRatio,
				Resources:          resources,
			},
			Job: &JobSpec{
				JarFile:                     &jarFile,
				Parallelism:                 &parallelism,
				MaxStateAgeToRestoreSeconds: &maxStateAgeToRestoreSeconds,
				RestartPolicy:               &restartPolicy,
				CleanupPolicy: &CleanupPolicy{
					AfterJobSucceeds: CleanupActionKeepCluster,
					AfterJobFails:    CleanupActionDeleteTaskManager,
				},
				Mode: &jobMode,
			},
			GCPConfig: &GCPConfig{
				ServiceAccount: &GCPServiceAccount{
					SecretName: "gcp-service-account-secret",
					KeyFile:    "gcp_service_account_key.json",
					MountPath:  "/etc/gcp_service_account",
				},
			},
			HadoopConfig: &HadoopConfig{
				ConfigMapName: "hadoop-configmap",
				MountPath:     "/etc/hadoop/conf",
			},
		},
	}
	var validator = &Validator{}
	var err = validator.ValidateCreate(&cluster)
	assert.NilError(t, err, "create validation failed unexpectedly")
}

func TestInvalidJobManagerSpec(t *testing.T) {
	var jmReplicas int32 = 1
	var rpcPort int32 = 8001
	var blobPort int32 = 8002
	var queryPort int32 = 8003
	var uiPort int32 = 8004

	cluster := FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: "default",
		},
		Spec: FlinkClusterSpec{
			FlinkVersion: "1.8",
			Image: ImageSpec{
				Name:       "flink:1.8.1",
				PullPolicy: corev1.PullPolicy("Always"),
			},
			JobManager: &JobManagerSpec{
				Replicas:    &jmReplicas,
				AccessScope: "XXX",
				Ports: JobManagerPorts{
					RPC:   &rpcPort,
					Blob:  &blobPort,
					Query: &queryPort,
					UI:    &uiPort,
				},
			},
		},
	}
	err := validator.ValidateCreate(&cluster)
	expectedErr := "jobmanager resource requests/limits are unspecified"
	assert.Equal(t, err.Error(), expectedErr)
}

func TestInvalidTaskManagerSpec(t *testing.T) {
	var jmReplicas int32 = DefaultJobManagerReplicas
	var tmReplicas int32 = DefaultTaskManagerReplicas
	var rpcPort int32 = 8001
	var blobPort int32 = 8002
	var queryPort int32 = 8003
	var uiPort int32 = 8004
	var dataPort int32 = 8005
	var memoryOffHeapRatio int32 = 25
	var memoryOffHeapMin = resource.MustParse("600M")
	resources := DefaultResources

	cluster := FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: "default",
		},
		Spec: FlinkClusterSpec{
			FlinkVersion: "1.8",
			Image: ImageSpec{
				Name:       "flink:1.8.1",
				PullPolicy: corev1.PullPolicy("Always"),
			},
			JobManager: &JobManagerSpec{
				Replicas:    &jmReplicas,
				AccessScope: AccessScopeVPC,
				Ports: JobManagerPorts{
					RPC:   &rpcPort,
					Blob:  &blobPort,
					Query: &queryPort,
					UI:    &uiPort,
				},
				MemoryOffHeapRatio: &memoryOffHeapRatio,
				MemoryOffHeapMin:   memoryOffHeapMin,
				Resources:          resources,
			},
			TaskManager: &TaskManagerSpec{
				Replicas: &tmReplicas,
				Ports: TaskManagerPorts{
					RPC:   &rpcPort,
					Data:  &dataPort,
					Query: &queryPort,
				},
				Resources: corev1.ResourceRequirements{
					Limits: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("500M"),
					},
				},
				MemoryOffHeapRatio: &memoryOffHeapRatio,
				MemoryOffHeapMin:   memoryOffHeapMin,
			},
		},
	}
	err := validator.ValidateCreate(&cluster)
	expectedErr := "invalid taskmanager memory configuration, memory limit must be larger than MemoryOffHeapMin, memory limit: 500000000 bytes, memoryOffHeapMin: 600000000 bytes"
	assert.Equal(t, err.Error(), expectedErr)
}

func TestInvalidJobSpec(t *testing.T) {
	var jmReplicas int32 = DefaultJobManagerReplicas
	var tmReplicas int32 = DefaultTaskManagerReplicas
	var rpcPort int32 = 8001
	var blobPort int32 = 8002
	var queryPort int32 = 8003
	var uiPort int32 = 8004
	var dataPort int32 = 8005
	var maxStateAgeToRestoreSeconds int32 = 300
	var restartPolicy = JobRestartPolicyFromSavepointOnFailure
	var validator = &Validator{}
	var memoryOffHeapRatio int32 = 25
	var memoryOffHeapMin = resource.MustParse("600M")
	resources := DefaultResources

	var cluster = FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: "default",
		},
		Spec: FlinkClusterSpec{
			FlinkVersion: "1.8",
			Image: ImageSpec{
				Name:       "flink:1.8.1",
				PullPolicy: corev1.PullPolicy("Always"),
			},
			JobManager: &JobManagerSpec{
				Replicas:    &jmReplicas,
				AccessScope: AccessScopeVPC,
				Ports: JobManagerPorts{
					RPC:   &rpcPort,
					Blob:  &blobPort,
					Query: &queryPort,
					UI:    &uiPort,
				},
				MemoryOffHeapRatio: &memoryOffHeapRatio,
				MemoryOffHeapMin:   memoryOffHeapMin,
				Resources:          resources,
			},
			TaskManager: &TaskManagerSpec{
				Replicas: &tmReplicas,
				Ports: TaskManagerPorts{
					RPC:   &rpcPort,
					Data:  &dataPort,
					Query: &queryPort,
				},
				MemoryOffHeapRatio: &memoryOffHeapRatio,
				MemoryOffHeapMin:   memoryOffHeapMin,
				Resources:          resources,
			},
			Job: &JobSpec{
				RestartPolicy:               &restartPolicy,
				MaxStateAgeToRestoreSeconds: &maxStateAgeToRestoreSeconds,
			},
		},
	}
	var err = validator.ValidateCreate(&cluster)
	var expectedErr = "job jarFile or pythonFile or pythonModule is unspecified"
	assert.Equal(t, err.Error(), expectedErr)

}

func TestUpdateStatusAllowed(t *testing.T) {
	var oldCluster = getSimpleFlinkCluster()
	var newCluster = getSimpleFlinkCluster()
	var validator = &Validator{}
	oldCluster.Status = FlinkClusterStatus{State: "NoReady"}
	newCluster.Status = FlinkClusterStatus{State: "Running"}
	err := validator.ValidateUpdate(&oldCluster, &newCluster)
	assert.NilError(t, err, "updating status failed unexpectedly")
}

func TestUpdateSavepointGeneration(t *testing.T) {
	var validator = &Validator{}
	var jarFile = "gs://my-bucket/myjob.jar"
	var jarFileNew = "gs://my-bucket/myjob-v2.jar"
	var parallelism int32 = 2
	var restartPolicy = JobRestartPolicyFromSavepointOnFailure
	var savepointDir = "/savepoint_dir"
	oldCluster := getSimpleFlinkCluster()
	oldCluster.Spec.Job = &JobSpec{
		JarFile:       &jarFile,
		Parallelism:   &parallelism,
		RestartPolicy: &restartPolicy,
		SavepointsDir: &savepointDir,
		CleanupPolicy: &CleanupPolicy{
			AfterJobSucceeds: CleanupActionKeepCluster,
			AfterJobFails:    CleanupActionDeleteTaskManager,
		},
	}
	oldCluster.Status = FlinkClusterStatus{
		Components: FlinkClusterComponentsStatus{
			Job: &JobStatus{
				SavepointGeneration: 2,
			},
		},
	}
	newCluster := getSimpleFlinkCluster()
	newCluster.Spec.Job = &JobSpec{
		SavepointGeneration: 4,
		JarFile:             &jarFile,
		Parallelism:         &parallelism,
		RestartPolicy:       &restartPolicy,
		SavepointsDir:       nil,
		CleanupPolicy: &CleanupPolicy{
			AfterJobSucceeds: CleanupActionKeepCluster,
			AfterJobFails:    CleanupActionDeleteTaskManager,
		},
	}
	err := validator.ValidateUpdate(&oldCluster, &newCluster)
	expectedErr := "you can only update savepointGeneration to 3"
	assert.Equal(t, err.Error(), expectedErr)

	newCluster = getSimpleFlinkCluster()
	newCluster.Spec.Job = &JobSpec{
		SavepointGeneration: 3,
		JarFile:             &jarFileNew,
		Parallelism:         &parallelism,
		RestartPolicy:       &restartPolicy,
		SavepointsDir:       &savepointDir,
		CleanupPolicy: &CleanupPolicy{
			AfterJobSucceeds: CleanupActionKeepCluster,
			AfterJobFails:    CleanupActionDeleteTaskManager,
		},
	}
	err = validator.ValidateUpdate(&oldCluster, &newCluster)
	expectedErr = "you cannot update savepointGeneration with others at the same time"
	assert.Equal(t, err.Error(), expectedErr)

	newCluster = getSimpleFlinkCluster()
	newCluster.Spec.Job = &JobSpec{
		SavepointGeneration: 3,
		JarFile:             &jarFile,
		Parallelism:         &parallelism,
		RestartPolicy:       &restartPolicy,
		SavepointsDir:       &savepointDir,
		CleanupPolicy: &CleanupPolicy{
			AfterJobSucceeds: CleanupActionKeepCluster,
			AfterJobFails:    CleanupActionDeleteTaskManager,
		},
	}
	err = validator.ValidateUpdate(&oldCluster, &newCluster)
	assert.Equal(t, err, nil)
}

func TestTaskManagerDeploymentTypeUpdate(t *testing.T) {
	// cannot update deploymentType
	var oldCluster = getSimpleFlinkCluster()
	var newCluster = getSimpleFlinkCluster()
	newCluster.Spec.TaskManager.DeploymentType = DeploymentTypeDeployment
	err := validator.ValidateUpdate(&oldCluster, &newCluster)
	expectedErr := "updating deploymentType is not allowed"
	assert.Equal(t, err.Error(), expectedErr)
}

func TestUpdateJob(t *testing.T) {
	var validator = &Validator{}
	var tc = &util.TimeConverter{}
	var maxStateAge = time.Duration(MaxStateAgeToRestore)
	var jarFileNew = "gs://my-bucket/myjob-v2.jar"

	// cannot remove savepointsDir
	var oldCluster = getSimpleFlinkCluster()
	var newCluster = getSimpleFlinkCluster()
	newCluster.Spec.Job.SavepointsDir = nil
	err := validator.ValidateUpdate(&oldCluster, &newCluster)
	expectedErr := "removing savepointsDir is not allowed"
	assert.Equal(t, err.Error(), expectedErr)

	// cannot change cluster type
	oldCluster = getSimpleFlinkCluster()
	newCluster = getSimpleFlinkCluster()
	newCluster.Spec.Job = nil
	err = validator.ValidateUpdate(&oldCluster, &newCluster)
	oldJson, _ := json.Marshal(&oldCluster.Spec.Job)
	newJson, _ := json.Marshal(&newCluster.Spec.Job)
	expectedErr = fmt.Sprintf("you cannot change cluster type between session cluster and job cluster, old spec.job: %q, new spec.job: %q", oldJson, newJson)
	assert.Equal(t, err.Error(), expectedErr)

	// cannot update when savepointDir is not provided
	oldCluster = getSimpleFlinkCluster()
	oldCluster.Spec.Job.SavepointsDir = nil
	newCluster = getSimpleFlinkCluster()
	newCluster.Spec.Job.SavepointsDir = nil
	newCluster.Spec.Job.JarFile = &jarFileNew
	err = validator.ValidateUpdate(&oldCluster, &newCluster)
	expectedErr = "updating job is not allowed when spec.job.savepointsDir was not provided"
	assert.Equal(t, err.Error(), expectedErr)

	// cannot update when takeSavepointOnUpdate is false and stale savepoint
	var takeSavepointOnUpdateFalse = false
	var savepointTime = time.Now().Add(-(maxStateAge + 10) * time.Second) // stale savepoint
	oldCluster = getSimpleFlinkCluster()
	oldCluster.Status.Components.Job = &JobStatus{
		SavepointTime:     tc.ToString(savepointTime),
		SavepointLocation: "gs://my-bucket/my-sp-123",
		State:             JobStateRunning,
	}
	newCluster = getSimpleFlinkCluster()
	newCluster.Spec.Job.JarFile = &jarFileNew
	newCluster.Spec.Job.TakeSavepointOnUpdate = &takeSavepointOnUpdateFalse
	err = validator.ValidateUpdate(&oldCluster, &newCluster)
	jobStatusJson, _ := json.Marshal(oldCluster.Status.Components.Job)
	expectedErr = fmt.Sprintf("cannot update spec: taking savepoint is skipped but no up-to-date savepoint, "+
		"spec.job.takeSavepointOnUpdate: false, spec.job.maxStateAgeToRestoreSeconds: 60, job status: %q", jobStatusJson)
	assert.Equal(t, err.Error(), expectedErr)

	// update when takeSavepointOnUpdate is false and savepoint is up-to-date
	takeSavepointOnUpdateFalse = false
	maxStateAge = time.Duration(*getSimpleFlinkCluster().Spec.Job.MaxStateAgeToRestoreSeconds)
	savepointTime = time.Now().Add(-(maxStateAge - 10) * time.Second) // up-to-date savepoint
	oldCluster = getSimpleFlinkCluster()
	oldCluster.Status.Components.Job = &JobStatus{
		SavepointTime:     tc.ToString(savepointTime),
		SavepointLocation: "gs://my-bucket/my-sp-123",
		State:             JobStateRunning,
	}
	newCluster = getSimpleFlinkCluster()
	newCluster.Spec.Job.JarFile = &jarFileNew
	newCluster.Spec.Job.TakeSavepointOnUpdate = &takeSavepointOnUpdateFalse
	err = validator.ValidateUpdate(&oldCluster, &newCluster)
	assert.Equal(t, err, nil)

	// spec update is allowed when takeSavepointOnUpdate is true and savepoint is not completed yet
	oldCluster = getSimpleFlinkCluster()
	oldCluster.Status.Components.Job = &JobStatus{
		FinalSavepoint:    false,
		SavepointLocation: "gs://my-bucket/my-sp-123",
		State:             JobStateRunning,
	}
	newCluster = getSimpleFlinkCluster()
	newCluster.Spec.Job.JarFile = &jarFileNew
	err = validator.ValidateUpdate(&oldCluster, &newCluster)
	assert.Equal(t, err, nil)

	// when job is stopped and no up-to-date savepoint
	var jobCompletionTime = time.Now()
	savepointTime = jobCompletionTime.Add(-(maxStateAge + 10) * time.Second) // stale savepoint
	oldCluster = getSimpleFlinkCluster()
	oldCluster.Status.Components.Job = &JobStatus{
		SavepointTime:     tc.ToString(savepointTime),
		SavepointLocation: "gs://my-bucket/my-sp-123",
		State:             JobStateFailed,
		CompletionTime:    &metav1.Time{Time: jobCompletionTime},
	}
	newCluster = getSimpleFlinkCluster()
	newCluster.Spec.Job.JarFile = &jarFileNew
	err = validator.ValidateUpdate(&oldCluster, &newCluster)
	jobStatusJson, _ = json.Marshal(oldCluster.Status.Components.Job)
	expectedErr = fmt.Sprintf("cannot update spec: taking savepoint is skipped but no up-to-date savepoint, "+
		"spec.job.takeSavepointOnUpdate: nil, spec.job.maxStateAgeToRestoreSeconds: 60, job status: %q", jobStatusJson)
	assert.Equal(t, err.Error(), expectedErr)

	// when job is stopped and savepoint is up-to-date
	jobCompletionTime = time.Now()
	savepointTime = jobCompletionTime.Add(-(maxStateAge - 10) * time.Second) // up-to-date savepoint
	oldCluster = getSimpleFlinkCluster()
	oldCluster.Status.Components.Job = &JobStatus{
		SavepointTime:     tc.ToString(savepointTime),
		SavepointLocation: "gs://my-bucket/my-sp-123",
		State:             JobStateFailed,
		CompletionTime:    &metav1.Time{Time: jobCompletionTime},
	}
	newCluster = getSimpleFlinkCluster()
	newCluster.Spec.Job.JarFile = &jarFileNew
	err = validator.ValidateUpdate(&oldCluster, &newCluster)
	assert.Equal(t, err, nil)

	// when job is stopped and savepoint is stale, but fromSavepoint is provided
	var fromSavepoint = "gs://my-bucket/sp-123"
	jobCompletionTime = time.Now()
	savepointTime = jobCompletionTime.Add(-(maxStateAge + 10) * time.Second) // stale savepoint
	oldCluster = getSimpleFlinkCluster()
	oldCluster.Status.Components.Job = &JobStatus{
		SavepointTime:     tc.ToString(savepointTime),
		SavepointLocation: "gs://my-bucket/my-sp-123",
		State:             JobStateFailed,
		CompletionTime:    &metav1.Time{Time: jobCompletionTime},
	}
	newCluster = getSimpleFlinkCluster()
	newCluster.Spec.Job.JarFile = &jarFileNew
	newCluster.Spec.Job.FromSavepoint = &fromSavepoint
	err = validator.ValidateUpdate(&oldCluster, &newCluster)
	assert.Equal(t, err, nil)
}

func TestUpdateCluster(t *testing.T) {
	var validator = &Validator{}
	var jmReplicas int32 = 1
	var rpcPort int32 = 8001
	var blobPort int32 = 8002
	var queryPort int32 = 8003
	var uiPort int32 = 8004
	var dataPort int32 = 8005
	var memoryOffHeapRatio int32 = 25
	var memoryOffHeapMin = resource.MustParse("600M")
	resources := DefaultResources

	oldCluster := getSimpleFlinkCluster()
	oldCluster.Spec.Image = ImageSpec{
		Name:       "flink:1.8.1",
		PullPolicy: corev1.PullAlways,
	}
	newCluster := getSimpleFlinkCluster()
	newCluster.Spec.Image = ImageSpec{
		Name:       "flink:1.9.3",
		PullPolicy: corev1.PullIfNotPresent,
	}
	err := validator.ValidateUpdate(&oldCluster, &newCluster)
	assert.NilError(t, err, "updating FlinkCluster image failed unexpectedly")

	oldCluster = getSimpleFlinkCluster()
	oldCluster.Spec.JobManager = &JobManagerSpec{
		Replicas:    &jmReplicas,
		AccessScope: AccessScopeVPC,
		Ports: JobManagerPorts{
			RPC:   &rpcPort,
			Blob:  &blobPort,
			Query: &queryPort,
			UI:    &uiPort,
		},
		MemoryOffHeapRatio: &memoryOffHeapRatio,
		MemoryOffHeapMin:   memoryOffHeapMin,
		Resources:          resources,
	}
	var newMemoryOffHeapRatio int32 = 20
	newCluster = getSimpleFlinkCluster()
	newCluster.Spec.JobManager = &JobManagerSpec{
		Replicas:    &jmReplicas,
		AccessScope: AccessScopeVPC,
		Ports: JobManagerPorts{
			RPC:   &rpcPort,
			Blob:  &blobPort,
			Query: &queryPort,
			UI:    &uiPort,
		},
		MemoryOffHeapRatio: &newMemoryOffHeapRatio,
		MemoryOffHeapMin:   memoryOffHeapMin,
		Resources:          resources,
	}
	err = validator.ValidateUpdate(&oldCluster, &newCluster)
	assert.NilError(t, err, "updating JobManger failed unexpectedly")

	var newTMReplicas int32 = 5
	newCluster = getSimpleFlinkCluster()
	newCluster.Spec.TaskManager = &TaskManagerSpec{
		Replicas: &newTMReplicas,
		Ports: TaskManagerPorts{
			RPC:   &rpcPort,
			Data:  &dataPort,
			Query: &queryPort,
		},
		MemoryOffHeapRatio: &memoryOffHeapRatio,
		MemoryOffHeapMin:   memoryOffHeapMin,
		Resources:          resources,
	}
	err = validator.ValidateUpdate(&oldCluster, &newCluster)
	assert.NilError(t, err, "updating TaskManger failed unexpectedly")
}

func TestInvalidGCPConfig(t *testing.T) {
	var gcpConfig = GCPConfig{
		ServiceAccount: &GCPServiceAccount{
			SecretName: "my-secret",
			KeyFile:    "my_service_account.json",
			MountPath:  "/etc/gcp/my_service_account.json",
		},
	}
	var validator = &Validator{}
	var err = validator.validateGCPConfig(&gcpConfig)
	var expectedErr = "invalid GCP service account volume mount path"
	assert.Assert(t, err != nil, "err is not expected to be nil")
	assert.Equal(t, err.Error(), expectedErr)
}

func TestUserControlSavepoint(t *testing.T) {
	var validator = &Validator{}
	var restartPolicy = JobRestartPolicyNever
	var savepointsDir = "gs://my-bucket/savepoints/"
	var newCluster = FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				ControlAnnotation: "savepoint",
			},
		},
	}

	var oldCluster1 = FlinkCluster{
		Spec:   FlinkClusterSpec{Job: &JobSpec{}},
		Status: FlinkClusterStatus{Control: &FlinkClusterControlStatus{State: ControlStateInProgress}},
	}
	var err1 = validator.ValidateUpdate(&oldCluster1, &newCluster)
	var expectedErr1 = "change is not allowed for control in progress, annotation: flinkclusters.flinkoperator.k8s.io/user-control"
	assert.Equal(t, err1.Error(), expectedErr1)

	var oldCluster2 = FlinkCluster{}
	var err2 = validator.ValidateUpdate(&oldCluster2, &newCluster)
	var expectedErr2 = "savepoint is not allowed for session cluster, annotation: flinkclusters.flinkoperator.k8s.io/user-control"
	assert.Equal(t, err2.Error(), expectedErr2)

	var oldCluster3 = FlinkCluster{Spec: FlinkClusterSpec{Job: &JobSpec{}}}
	var err3 = validator.ValidateUpdate(&oldCluster3, &newCluster)
	var expectedErr3 = "savepoint is not allowed without spec.job.savepointsDir, annotation: flinkclusters.flinkoperator.k8s.io/user-control"
	assert.Equal(t, err3.Error(), expectedErr3)

	var oldCluster4 = FlinkCluster{Spec: FlinkClusterSpec{Job: &JobSpec{SavepointsDir: &savepointsDir}}}
	var err4 = validator.ValidateUpdate(&oldCluster4, &newCluster)
	var expectedErr4 = "savepoint is not allowed because job is not started yet or already stopped, annotation: flinkclusters.flinkoperator.k8s.io/user-control"
	assert.Equal(t, err4.Error(), expectedErr4)

	var oldCluster5 = FlinkCluster{
		Spec:   FlinkClusterSpec{Job: &JobSpec{SavepointsDir: &savepointsDir}},
		Status: FlinkClusterStatus{Components: FlinkClusterComponentsStatus{Job: &JobStatus{State: JobStateSucceeded}}},
	}
	var err5 = validator.ValidateUpdate(&oldCluster5, &newCluster)
	var expectedErr5 = "savepoint is not allowed because job is not started yet or already stopped, annotation: flinkclusters.flinkoperator.k8s.io/user-control"
	assert.Equal(t, err5.Error(), expectedErr5)

	var oldCluster6 = FlinkCluster{
		Spec:   FlinkClusterSpec{Job: &JobSpec{RestartPolicy: &restartPolicy, SavepointsDir: &savepointsDir}},
		Status: FlinkClusterStatus{Components: FlinkClusterComponentsStatus{Job: &JobStatus{State: JobStateFailed}}},
	}
	var err6 = validator.ValidateUpdate(&oldCluster6, &newCluster)
	var expectedErr6 = "savepoint is not allowed because job is not started yet or already stopped, annotation: flinkclusters.flinkoperator.k8s.io/user-control"
	assert.Equal(t, err6.Error(), expectedErr6)
}

func TestUserControlJobCancel(t *testing.T) {
	var validator = &Validator{}
	var restartPolicy = JobRestartPolicyNever
	var newCluster = FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				ControlAnnotation: "job-cancel",
			},
		},
	}

	var oldCluster1 = FlinkCluster{
		Spec:   FlinkClusterSpec{Job: &JobSpec{}},
		Status: FlinkClusterStatus{Control: &FlinkClusterControlStatus{State: ControlStateInProgress}},
	}
	var err1 = validator.ValidateUpdate(&oldCluster1, &newCluster)
	var expectedErr1 = "change is not allowed for control in progress, annotation: flinkclusters.flinkoperator.k8s.io/user-control"
	assert.Equal(t, err1.Error(), expectedErr1)

	var oldCluster2 = FlinkCluster{}
	var err2 = validator.ValidateUpdate(&oldCluster2, &newCluster)
	var expectedErr2 = "job-cancel is not allowed for session cluster, annotation: flinkclusters.flinkoperator.k8s.io/user-control"
	assert.Equal(t, err2.Error(), expectedErr2)

	var oldCluster3 = FlinkCluster{Spec: FlinkClusterSpec{Job: &JobSpec{}}}
	var err3 = validator.ValidateUpdate(&oldCluster3, &newCluster)
	var expectedErr3 = "job-cancel is not allowed because job is not started yet or already terminated, annotation: flinkclusters.flinkoperator.k8s.io/user-control"
	assert.Equal(t, err3.Error(), expectedErr3)

	var oldCluster4 = FlinkCluster{
		Spec: FlinkClusterSpec{Job: &JobSpec{}},
		Status: FlinkClusterStatus{Components: FlinkClusterComponentsStatus{Job: &JobStatus{
			State:          JobStateSucceeded,
			CompletionTime: &metav1.Time{Time: time.Now()},
		}}},
	}
	var err4 = validator.ValidateUpdate(&oldCluster4, &newCluster)
	var expectedErr4 = "job-cancel is not allowed because job is not started yet or already terminated, annotation: flinkclusters.flinkoperator.k8s.io/user-control"
	assert.Equal(t, err4.Error(), expectedErr4)

	var oldCluster5 = FlinkCluster{
		Spec: FlinkClusterSpec{Job: &JobSpec{RestartPolicy: &restartPolicy}},
		Status: FlinkClusterStatus{Components: FlinkClusterComponentsStatus{
			Job: &JobStatus{State: JobStateFailed,
				CompletionTime: &metav1.Time{Time: time.Now()},
			}}},
	}
	var err5 = validator.ValidateUpdate(&oldCluster5, &newCluster)
	var expectedErr5 = "job-cancel is not allowed because job is not started yet or already terminated, annotation: flinkclusters.flinkoperator.k8s.io/user-control"
	assert.Equal(t, err5.Error(), expectedErr5)
}

func TestUserControlInvalid(t *testing.T) {
	var validator = &Validator{}
	var newCluster = FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				ControlAnnotation: "cancel",
			},
		},
	}
	var oldCluster = FlinkCluster{}
	var err = validator.ValidateUpdate(&oldCluster, &newCluster)
	var expectedErr = "invalid value for annotation key: flinkclusters.flinkoperator.k8s.io/user-control, value: cancel, available values: savepoint, job-cancel"
	assert.Equal(t, err.Error(), expectedErr)
}

func TestDupPort(t *testing.T) {
	var jmReplicas int32 = 1
	var rpcPort int32 = 8001
	var blobPort int32 = 8002
	var queryPort int32 = 8003
	var uiPort int32 = 8004
	var flinkPorts = JobManagerPorts{
		RPC:   &rpcPort,
		Blob:  &blobPort,
		Query: &queryPort,
		UI:    &uiPort,
	}
	var jm = &JobManagerSpec{Replicas: &jmReplicas, AccessScope: AccessScopeVPC, Ports: flinkPorts,
		ExtraPorts: []NamedPort{
			{Name: "rpc", ContainerPort: 9001}}}
	var err = validator.validateJobManager(nil, jm)
	var expectedErr = "duplicate port name rpc in jobmanager, each port name of ports and extraPorts must be unique"
	assert.Equal(t, err.Error(), expectedErr)

	jm = &JobManagerSpec{Replicas: &jmReplicas, AccessScope: AccessScopeVPC, Ports: flinkPorts,
		ExtraPorts: []NamedPort{
			{Name: "monitoring", ContainerPort: 9249},
			{Name: "monitoring", ContainerPort: 9259}}}
	err = validator.validateJobManager(nil, jm)
	expectedErr = "duplicate port name monitoring in jobmanager, each port name of ports and extraPorts must be unique"
	assert.Equal(t, err.Error(), expectedErr)

	jm = &JobManagerSpec{Replicas: &jmReplicas, AccessScope: AccessScopeVPC, Ports: flinkPorts,
		ExtraPorts: []NamedPort{
			{Name: "rpc2", ContainerPort: 8001}}}
	err = validator.validateJobManager(nil, jm)
	expectedErr = "duplicate containerPort 8001 in jobmanager, each port number of ports and extraPorts must be unique"
	assert.Equal(t, err.Error(), expectedErr)

	jm = &JobManagerSpec{Replicas: &jmReplicas, AccessScope: AccessScopeVPC, Ports: flinkPorts,
		ExtraPorts: []NamedPort{
			{Name: "monitoring", ContainerPort: 9249},
			{Name: "prometheus", ContainerPort: 9249}}}
	err = validator.validateJobManager(nil, jm)
	expectedErr = "duplicate containerPort 9249 in jobmanager, each port number of ports and extraPorts must be unique"
	assert.Equal(t, err.Error(), expectedErr)
}

func getSimpleFlinkCluster() FlinkCluster {
	var jmReplicas int32 = DefaultJobManagerReplicas
	var tmReplicas int32 = DefaultTaskManagerReplicas
	var rpcPort int32 = 8001
	var blobPort int32 = 8002
	var queryPort int32 = 8003
	var uiPort int32 = 8004
	var dataPort int32 = 8005
	var memoryOffHeapRatio int32 = 25
	var memoryOffHeapMin = resource.MustParse("600M")
	var jarFile = "gs://my-bucket/myjob.jar"
	var parallelism int32 = 2
	var maxStateAge = MaxStateAgeToRestore
	var restartPolicy = JobRestartPolicyFromSavepointOnFailure
	var savepointDir = "/savepoint_dir"
	var jobMode = JobModeDetached
	resources := DefaultResources
	return FlinkCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: "default",
		},
		Spec: FlinkClusterSpec{
			FlinkVersion: "1.8",
			Image: ImageSpec{
				Name:       "flink:1.8.1",
				PullPolicy: corev1.PullPolicy("Always"),
			},
			JobManager: &JobManagerSpec{
				Replicas:    &jmReplicas,
				AccessScope: AccessScopeVPC,
				Ports: JobManagerPorts{
					RPC:   &rpcPort,
					Blob:  &blobPort,
					Query: &queryPort,
					UI:    &uiPort,
				},
				MemoryOffHeapRatio: &memoryOffHeapRatio,
				MemoryOffHeapMin:   memoryOffHeapMin,
				Resources:          resources,
			},
			TaskManager: &TaskManagerSpec{
				Replicas: &tmReplicas,
				Ports: TaskManagerPorts{
					RPC:   &rpcPort,
					Data:  &dataPort,
					Query: &queryPort,
				},
				MemoryOffHeapRatio: &memoryOffHeapRatio,
				MemoryOffHeapMin:   memoryOffHeapMin,
				Resources:          resources,
			},
			Job: &JobSpec{
				JarFile:                     &jarFile,
				Parallelism:                 &parallelism,
				MaxStateAgeToRestoreSeconds: &maxStateAge,
				RestartPolicy:               &restartPolicy,
				SavepointsDir:               &savepointDir,
				CleanupPolicy: &CleanupPolicy{
					AfterJobSucceeds: CleanupActionKeepCluster,
					AfterJobFails:    CleanupActionDeleteTaskManager,
				},
				Mode: &jobMode,
			},
		},
	}
}

func TestFlinkClusterValidation(t *testing.T) {
	longName := strings.Repeat("a", 254)

	invalidJobManagerAnnotations := func() *FlinkCluster {
		cluster := getSimpleFlinkCluster()
		cluster.Spec.JobManager.PodAnnotations = map[string]string{
			longName: "bar",
		}
		return &cluster
	}
	invalidJobManagerLabels := func() *FlinkCluster {
		cluster := getSimpleFlinkCluster()
		cluster.Spec.JobManager.PodLabels = map[string]string{
			longName: "bar",
		}
		return &cluster
	}
	invalidTaskManagerAnnotations := func() *FlinkCluster {
		cluster := getSimpleFlinkCluster()
		cluster.Spec.TaskManager.PodAnnotations = map[string]string{
			longName: "bar",
		}
		return &cluster
	}
	invalidTaskManagerLabels := func() *FlinkCluster {
		cluster := getSimpleFlinkCluster()
		cluster.Spec.TaskManager.PodLabels = map[string]string{
			longName: "bar",
		}
		return &cluster
	}
	invalidJobAnnotations := func() *FlinkCluster {
		cluster := getSimpleFlinkCluster()
		cluster.Spec.Job.PodAnnotations = map[string]string{
			longName: "bar",
		}
		return &cluster
	}
	invalidJobLabels := func() *FlinkCluster {
		cluster := getSimpleFlinkCluster()
		cluster.Spec.Job.PodLabels = map[string]string{
			longName: "bar",
		}
		return &cluster
	}
	invalidClusterName := func() *FlinkCluster {
		cluster := getSimpleFlinkCluster()
		cluster.Name = "1-invalid-name"
		return &cluster
	}
	invalidLongClusterName := func() *FlinkCluster {
		cluster := getSimpleFlinkCluster()
		cluster.Name = longName
		return &cluster
	}

	data := []struct {
		testName    string
		run         func() *FlinkCluster
		expectedErr string
	}{
		{
			"invalid jm annotations",
			invalidJobManagerAnnotations,
			fmt.Sprintf("spec.jobManager.podAnnotations: Invalid value: \"%s\": name part must be no more than 63 characters", longName),
		},
		{
			"invalid jm labels",
			invalidJobManagerLabels,
			fmt.Sprintf("spec.jobManager.podLabels: Invalid value: \"%s\": name part must be no more than 63 characters", longName),
		},
		{
			"invalid tm annotations",
			invalidTaskManagerAnnotations,
			fmt.Sprintf("spec.taskManager.podAnnotations: Invalid value: \"%s\": name part must be no more than 63 characters", longName),
		},
		{
			"invalid tm labels",
			invalidTaskManagerLabels,
			fmt.Sprintf("spec.taskManager.podLabels: Invalid value: \"%s\": name part must be no more than 63 characters", longName),
		},
		{
			"invalid job annotations",
			invalidJobAnnotations,
			fmt.Sprintf("spec.job.podAnnotations: Invalid value: \"%s\": name part must be no more than 63 characters", longName),
		},
		{
			"invalid job labels",
			invalidJobLabels,
			fmt.Sprintf("spec.job.podLabels: Invalid value: \"%s\": name part must be no more than 63 characters", longName),
		},
		{
			"invalid cluster name",
			invalidClusterName,
			fmt.Sprintf(dns1035ErrorMsg, "1-invalid-name"),
		},
		{
			"invalid cluster long name",
			invalidLongClusterName,
			"cluster name size needs to greater than 0 and less than 50",
		},
	}

	for _, tt := range data {
		t.Run(tt.testName, func(t *testing.T) {
			err := validator.ValidateCreate(tt.run())
			assert.Error(t, err, tt.expectedErr)
		})
	}
}
