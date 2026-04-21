package vos

import "testing"

func mustFingerprint(value string) Fingerprint {
	fp, err := NewFingerprint(value)
	if err != nil {
		panic(err)
	}
	return fp
}

func mustEnvironment(value string) Environment {
	env, err := NewEnvironment(value)
	if err != nil {
		panic(err)
	}
	return env
}

func TestFingerprintSet_Get(t *testing.T) {
	fps := map[FingerprintKind]Fingerprint{
		KindCode: mustFingerprint("abc123"),
		KindVars: mustFingerprint("def456"),
	}
	env := mustEnvironment("prod")
	fs := NewFingerprintSet(fps, env)

	t.Run("kind presente", func(t *testing.T) {
		fp, ok := fs.Get(KindCode)
		if !ok {
			t.Fatal("esperaba encontrar KindCode")
		}
		if fp.String() != "abc123" {
			t.Errorf("valor incorrecto: %q", fp.String())
		}
	})

	t.Run("kind ausente es legítimo", func(t *testing.T) {
		_, ok := fs.Get(KindInstruction)
		if ok {
			t.Error("KindInstruction no debería estar presente")
		}
	})

	t.Run("ambiente", func(t *testing.T) {
		if !fs.Environment().Equals(env) {
			t.Error("ambiente incorrecto")
		}
	})
}

func TestFingerprintSet_ImmutableCopy(t *testing.T) {
	fps := map[FingerprintKind]Fingerprint{
		KindCode: mustFingerprint("aaa"),
	}
	env := mustEnvironment("sand")
	fs := NewFingerprintSet(fps, env)

	// Modificar el mapa original no debe afectar al set
	fps[KindVars] = mustFingerprint("bbb")
	_, ok := fs.Get(KindVars)
	if ok {
		t.Error("el set no debe verse afectado por modificaciones al mapa original")
	}
}
