package elbv2

import "testing"

// TestApplyTargetGroupTupleField_HappyPath confirms the parser stores a
// canonical (TargetGroupArn, Weight) tuple at the supplied index.
func TestApplyTargetGroupTupleField_HappyPath(t *testing.T) {
	cfg := &ForwardActionConfig{}

	applyTargetGroupTupleField(cfg, "1.TargetGroupArn", "arn:tg/v1")
	applyTargetGroupTupleField(cfg, "1.Weight", "90")
	applyTargetGroupTupleField(cfg, "2.TargetGroupArn", "arn:tg/v2")
	applyTargetGroupTupleField(cfg, "2.Weight", "10")

	if got := len(cfg.TargetGroups); got != 2 {
		t.Fatalf("len(TargetGroups) = %d, want 2", got)
	}

	if got := cfg.TargetGroups[0]; got.TargetGroupArn != "arn:tg/v1" || got.Weight != 90 {
		t.Errorf("[0] = %+v, want {arn:tg/v1, 90}", got)
	}

	if got := cfg.TargetGroups[1]; got.TargetGroupArn != "arn:tg/v2" || got.Weight != 10 {
		t.Errorf("[1] = %+v, want {arn:tg/v2, 10}", got)
	}
}

// TestApplyTargetGroupTupleField_RejectsHugeIndex is the regression
// guard for the slice-grow DoS: a hostile member.N where N is huge
// would previously allocate billions of zero-valued tuples (~24 GB at
// member.1e9). The cap drops the entry without growing the slice.
func TestApplyTargetGroupTupleField_RejectsHugeIndex(t *testing.T) {
	cfg := &ForwardActionConfig{}

	applyTargetGroupTupleField(cfg, "1000000000.TargetGroupArn", "arn:dos")

	if len(cfg.TargetGroups) != 0 {
		t.Errorf("hostile member.N expanded slice to %d entries; expected drop", len(cfg.TargetGroups))
	}
}

// TestApplyTargetGroupTupleField_RejectsAtBoundary checks the cap is
// inclusive on the way in: index 100 lands, index 101 is dropped.
func TestApplyTargetGroupTupleField_RejectsAtBoundary(t *testing.T) {
	t.Run("index_at_cap_lands", func(t *testing.T) {
		cfg := &ForwardActionConfig{}
		applyTargetGroupTupleField(cfg, "100.TargetGroupArn", "arn:edge")

		if len(cfg.TargetGroups) != maxTargetGroupTuples {
			t.Errorf("index=cap should land; len=%d", len(cfg.TargetGroups))
		}
	})

	t.Run("index_above_cap_dropped", func(t *testing.T) {
		cfg := &ForwardActionConfig{}
		applyTargetGroupTupleField(cfg, "101.TargetGroupArn", "arn:over")

		if len(cfg.TargetGroups) != 0 {
			t.Errorf("index>cap should drop; len=%d", len(cfg.TargetGroups))
		}
	})
}

// TestApplyTargetGroupTupleField_RejectsZeroAndNegativeIndex covers the
// idx<1 guard.
func TestApplyTargetGroupTupleField_RejectsZeroAndNegativeIndex(t *testing.T) {
	for _, raw := range []string{"0", "-1", "abc", ""} {
		cfg := &ForwardActionConfig{}
		applyTargetGroupTupleField(cfg, raw+".Weight", "50")

		if len(cfg.TargetGroups) != 0 {
			t.Errorf("idx=%q should be dropped; got len=%d", raw, len(cfg.TargetGroups))
		}
	}
}

// TestApplyTargetGroupTupleField_WeightBounds confirms the parser
// silently drops weights outside the AWS-published [0, 1000] range
// rather than letting them poison storage.
func TestApplyTargetGroupTupleField_WeightBounds(t *testing.T) {
	for _, tc := range []struct {
		name   string
		weight string
		want   int
	}{
		{"zero_lands", "0", 0},
		{"max_lands", "1000", 1000},
		{"negative_dropped", "-1", 0},
		{"above_max_dropped", "1001", 0},
		{"int_overflow_dropped", "9999999999", 0},
		{"non_numeric_dropped", "many", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &ForwardActionConfig{}
			applyTargetGroupTupleField(cfg, "1.Weight", tc.weight)

			if got := cfg.TargetGroups[0].Weight; got != tc.want {
				t.Errorf("Weight = %d, want %d", got, tc.want)
			}
		})
	}
}
