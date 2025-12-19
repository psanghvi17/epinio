package application

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

func TestPVCMatchesConfigValid(t *testing.T) {
	config := StagingStorageValues{
		Size:             "1Gi",
		AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
		VolumeMode:       v1.PersistentVolumeFilesystem,
		StorageClassName: "fast",
	}
	pvc := v1.PersistentVolumeClaim{
		Status: v1.PersistentVolumeClaimStatus{Phase: v1.ClaimBound},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			VolumeMode:  ptr.To(v1.PersistentVolumeFilesystem),
			Resources: v1.VolumeResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			StorageClassName: ptr.To("fast"),
		},
	}

	valid, reason := pvcMatchesConfig(pvc, config)
	if !valid {
		t.Fatalf("expected pvc to be valid, got reason %s", reason)
	}
}

func TestPVCMatchesConfigSizeMismatch(t *testing.T) {
	config := StagingStorageValues{
		Size:        "2Gi",
		AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
		VolumeMode:  v1.PersistentVolumeFilesystem,
	}
	pvc := v1.PersistentVolumeClaim{
		Status: v1.PersistentVolumeClaimStatus{Phase: v1.ClaimBound},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			VolumeMode:  ptr.To(v1.PersistentVolumeFilesystem),
			Resources: v1.VolumeResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}

	valid, _ := pvcMatchesConfig(pvc, config)
	if valid {
		t.Fatalf("expected pvc to be invalid due to size mismatch")
	}
}

func TestPVCMatchesConfigStorageClassMismatch(t *testing.T) {
	config := StagingStorageValues{
		Size:             "1Gi",
		AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
		VolumeMode:       v1.PersistentVolumeFilesystem,
		StorageClassName: "fast",
	}
	pvc := v1.PersistentVolumeClaim{
		Status: v1.PersistentVolumeClaimStatus{Phase: v1.ClaimBound},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			VolumeMode:  ptr.To(v1.PersistentVolumeFilesystem),
			Resources: v1.VolumeResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			StorageClassName: ptr.To("slow"),
		},
	}

	valid, _ := pvcMatchesConfig(pvc, config)
	if valid {
		t.Fatalf("expected pvc to be invalid due to storage class mismatch")
	}
}

func TestAccessModesEqualOrderAgnostic(t *testing.T) {
	current := []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce, v1.ReadOnlyMany}
	desired := []v1.PersistentVolumeAccessMode{v1.ReadOnlyMany, v1.ReadWriteOnce}

	if !accessModesEqual(current, desired) {
		t.Fatalf("expected access modes to be equal regardless of order")
	}
}

func TestAccessModesEqualEmptyDesired(t *testing.T) {
	current := []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}
	desired := []v1.PersistentVolumeAccessMode{}

	if accessModesEqual(current, desired) {
		t.Fatalf("expected mismatch when desired modes are empty but current are not")
	}
}

func TestPVCMatchesConfigPendingPhase(t *testing.T) {
	config := StagingStorageValues{
		Size:        "1Gi",
		AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
		VolumeMode:  v1.PersistentVolumeFilesystem,
	}
	pvc := v1.PersistentVolumeClaim{
		Status: v1.PersistentVolumeClaimStatus{Phase: v1.ClaimPending},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			VolumeMode:  ptr.To(v1.PersistentVolumeFilesystem),
			Resources: v1.VolumeResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}

	valid, reason := pvcMatchesConfig(pvc, config)
	if valid || reason == "" {
		t.Fatalf("expected pending pvc to be invalid with reason, got valid=%t reason=%s", valid, reason)
	}
}

func TestApplyPVCDefaultsNilSafe(t *testing.T) {
	applyPVCDefaults(nil)
}

func TestApplyPVCDefaultsSetsValues(t *testing.T) {
	cfg := StagingStorageValues{}
	applyPVCDefaults(&cfg)

	if cfg.Size != "1Gi" {
		t.Fatalf("expected default size 1Gi, got %s", cfg.Size)
	}
	if len(cfg.AccessModes) != 1 || cfg.AccessModes[0] != v1.ReadWriteOnce {
		t.Fatalf("expected default access mode ReadWriteOnce, got %v", cfg.AccessModes)
	}
	if cfg.VolumeMode != v1.PersistentVolumeFilesystem {
		t.Fatalf("expected default volume mode Filesystem, got %s", cfg.VolumeMode)
	}
}
