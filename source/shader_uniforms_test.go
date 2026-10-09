package main

import "testing"

func cloneShaderScalarTestUniforms(source map[string]any) map[string]any {
	uniforms := make(map[string]any, len(source))
	for name, value := range source {
		if values, ok := value.([]float32); ok && len(values) == 1 {
			uniforms[name] = []float32{values[0]}
		} else {
			uniforms[name] = value
		}
	}
	return uniforms
}

func TestShaderScalarUpdatesReuseBoundStorage(t *testing.T) {
	variant := newLightingShaderVariant(nil, 8, 8, 8)
	before := variant.uniforms["NightFactor"].([]float32)
	if allocs := testing.AllocsPerRun(100, func() {
		updateLightingScalarFixture(variant.uniforms, 0.2)
	}); allocs != 0 {
		t.Fatalf("lighting scalar updates allocated %g times", allocs)
	}
	if after := variant.op.Uniforms["NightFactor"].([]float32); &after[0] != &before[0] || after[0] != 0.7 {
		t.Fatal("scalar updates replaced bound storage or did not reach draw options")
	}
	for kind := replacementEffectHealing; kind <= replacementEffectCoinReward; kind++ {
		uniforms := cloneShaderScalarTestUniforms(replacementEffectShaderStates[kind].uniforms)
		for name, value := range uniforms {
			if _, scalar := value.(float32); scalar {
				t.Fatalf("effect %d scalar %s was boxed instead of bound once", kind, name)
			}
		}
		if allocs := testing.AllocsPerRun(100, func() {
			updateReplacementScalarFixture(uniforms, 0.2)
		}); allocs != 0 {
			t.Fatalf("effect %d scalar updates allocated %g times", kind, allocs)
		}
	}
}

func updateLightingScalarFixture(uniforms map[string]any, phase float32) {
	setShaderFloat32(uniforms, "LightStrength", 1.2+phase)
	setShaderFloat32(uniforms, "GlowStrength", 0.8+phase)
	setShaderFloat32(uniforms, "MaxLightPlane", 7+phase)
	setShaderFloat32(uniforms, "NightFactor", 0.5+phase)
	setShaderFloat32(uniforms, "HasCharacterShadowMask", 0)
}

func updateReplacementScalarFixture(uniforms map[string]any, phase float32) {
	setShaderFloat32(uniforms, "Phase", phase)
	setShaderFloat32(uniforms, "Alpha", 0.8+phase)
	setShaderFloat32(uniforms, "Energy", 0.5+phase)
	setShaderFloat32(uniforms, "HasMask", 1+phase)
	setShaderFloat32(uniforms, "HasReflection", 0)
	setShaderFloat32(uniforms, "HasPoolOutline", 0)
	setShaderFloat32(uniforms, "MaskInvScale", 0.5+phase)
	setShaderFloat32(uniforms, "SpriteLightOnly", 0)
}

func BenchmarkShaderScalarUniformUpdates(b *testing.B) {
	b.Run("Lighting", func(b *testing.B) {
		variant := newLightingShaderVariant(nil, 8, 8, 8)
		phase := float32(0.2)
		b.ReportAllocs()
		for b.Loop() {
			updateLightingScalarFixture(variant.uniforms, phase)
			phase += 0.00001
		}
	})
	b.Run("ReplacementEffect", func(b *testing.B) {
		uniforms := cloneShaderScalarTestUniforms(replacementEffectShaderStates[replacementEffectHealing].uniforms)
		phase := float32(0.2)
		b.ReportAllocs()
		for b.Loop() {
			updateReplacementScalarFixture(uniforms, phase)
			phase += 0.00001
		}
	})
}
