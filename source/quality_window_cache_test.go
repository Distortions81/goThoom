package main

import (
	"testing"
	"time"

	"gothoom/eui"
)

func TestQualityWindowUsesRenderCache(t *testing.T) {
	initFont()
	originalWindow := qualityWin
	qualityWin = nil
	t.Cleanup(func() {
		if qualityWin != nil {
			qualityWin.RemoveWindow()
		}
		qualityWin = originalWindow
	})

	makeQualityWindow()
	if qualityWin.NoCache {
		t.Fatal("quality window redraws its full contents every frame")
	}
	if qualityWin.RefreshInterval() != 100*time.Millisecond {
		t.Fatalf("quality refresh interval = %v, want 100ms", qualityWin.RefreshInterval())
	}
}

func TestQualityWindowGroupsAndDisablesShaderEffects(t *testing.T) {
	initFont()
	originalSettings := gs
	originalWindow := qualityWin
	qualityWin = nil
	gs = gsdef
	gs.ShadersEnabled = false
	t.Cleanup(func() {
		if qualityWin != nil {
			qualityWin.RemoveWindow()
		}
		qualityWin = originalWindow
		gs = originalSettings
	})

	makeQualityWindow()
	if shadersEnabledCB == nil || shadersEnabledCB.Disabled {
		t.Fatal("shader master is missing or disabled")
	}
	for name, control := range map[string]*eui.ItemData{
		"artwork upscaling": upscaleModeDD,
		"lighting":          shaderLightingCB,
		"magic effects":     replacementEffectsCB,
		"mobile blending":   animCB,
		"world blending":    pictBlendCB,
	} {
		if control == nil || !control.Disabled {
			t.Errorf("%s control was not greyed out by the shader master", name)
		}
	}
	if motionCB == nil || motionCB.Disabled {
		t.Fatal("CPU motion smoothing was incorrectly disabled with shaders")
	}

	contains := func(root *eui.ItemData, target *eui.ItemData) bool {
		var visit func(*eui.ItemData) bool
		visit = func(item *eui.ItemData) bool {
			if item == target {
				return true
			}
			for _, child := range item.Contents {
				if visit(child) {
					return true
				}
			}
			return false
		}
		return visit(root)
	}
	var shaderSection *eui.ItemData
	var findSection func(*eui.ItemData)
	findSection = func(item *eui.ItemData) {
		if shaderSection != nil {
			return
		}
		for _, child := range item.Contents {
			if child.Text == "Shader Effects" && contains(item, shadersEnabledCB) {
				shaderSection = item
				return
			}
		}
		for _, child := range item.Contents {
			findSection(child)
		}
	}
	for _, item := range qualityWin.Contents {
		findSection(item)
	}
	if shaderSection == nil {
		t.Fatal("quality window is missing the Shader Effects group")
	}
	for _, control := range []*eui.ItemData{upscaleModeDD, shaderLightingCB, replacementEffectsCB, animCB, pictBlendCB} {
		if !contains(shaderSection, control) {
			t.Errorf("shader-backed control %q is outside the Shader Effects group", control.Text+control.Label)
		}
	}
}
