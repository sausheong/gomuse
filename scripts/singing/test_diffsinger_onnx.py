import unittest
import numpy as np
from diffsinger_onnx import prepare_phrase, pronunciation, check_inputs, VariancePredictor


class PhonemeTests(unittest.TestCase):
    dictionary = {"stay": [["S", "T", "EY1"]], "me": [["M", "IY1"]]}
    table = {"SP": 0, "s": 1, "t": 2, "ey": 3, "m": 4, "iy": 5}

    def note(self, start, lyric, midi=60, duration=0.5):
        return dict(start=start, lyric=lyric, midi=midi, duration=duration)

    def test_melisma_preserves_vowel_and_changes_pitch(self):
        origin, tokens, durations, f0, symbols = prepare_phrase(
            [self.note(0.5, "stay"), self.note(1, "+", 64), self.note(1.5, "me")],
            self.table, 0.01, self.dictionary)
        self.assertEqual(symbols, ["SP", "s", "t", "ey", "m", "iy", "SP"])
        self.assertEqual(int(durations.sum()), f0.shape[1])
        self.assertAlmostEqual(float(f0[0, 50]), 261.6256, places=3)
        self.assertAlmostEqual(float(f0[0, 100]), 329.6276, places=3)
        self.assertEqual(origin, 0.25)

    def test_unknown_word_fails_instead_of_guessing(self):
        with self.assertRaisesRegex(ValueError, "Unknown word"):
            pronunciation("not-in-dictionary", self.dictionary)

    def test_explicit_phonemes(self):
        self.assertEqual(pronunciation("[s t ey]", {}), (["s", "t", "ey"], 2))

    def test_multiple_syllables_need_explicit_alignment(self):
        with self.assertRaisesRegex(ValueError, "one syllable"):
            pronunciation("[hh eh l ow]", {})

    def test_missing_voicebank_phone_fails(self):
        with self.assertRaisesRegex(ValueError, "no phoneme"):
            prepare_phrase([self.note(0, "me")], {"SP": 0}, 0.01, self.dictionary)

    def test_language_prefixed_phonemes(self):
        table = {"SP": 0, "en/m": 1, "en/iy": 2}
        *_, symbols = prepare_phrase([self.note(0, "me")], table, 0.01, self.dictionary)
        self.assertEqual(symbols, ["SP", "en/m", "en/iy", "SP"])

    def test_short_notes_keep_positive_frame_durations(self):
        _, _, durations, f0, _ = prepare_phrase([self.note(0, "stay", duration=0.08)], self.table, 512/44100, self.dictionary)
        self.assertTrue(np.all(durations > 0))
        self.assertEqual(int(durations.sum()), f0.shape[1])

    def test_unsupported_model_inputs_fail(self):
        class Input:
            name = "unknown_conditioning"
        class Model:
            def get_inputs(self): return [Input()]
        with self.assertRaisesRegex(ValueError, "Unsupported model inputs"):
            check_inputs(Model(), {})

    def test_variance_retake_and_pitch_units(self):
        class Field:
            def __init__(self, name): self.name = name

        class Encoder:
            def get_inputs(self): return [Field(n) for n in ("tokens", "ph_dur")]
            def run(self, _, values):
                np.testing.assert_array_equal(values["tokens"], [[2, 7]])
                return [np.zeros((1, 2, 256), dtype=np.float32)]

        class Predictor:
            def get_inputs(self):
                return [Field(n) for n in ("encoder_out", "ph_dur", "pitch", "retake", "tension", "steps", "spk_embed")]
            def get_outputs(self): return [Field("tension_pred")]
            def run(self, _, values):
                np.testing.assert_allclose(values["pitch"], [[69, 81]], atol=1e-5)
                np.testing.assert_array_equal(values["retake"], [[[True], [True]]])
                assert values["spk_embed"].shape == (1, 2, 256)
                return [np.array([[0.2, 0.3]], dtype=np.float32)]

        variance = VariancePredictor.__new__(VariancePredictor)
        variance.table = {"SP": 2, "iy": 7}
        variance.encoder, variance.predictor = Encoder(), Predictor()
        variance.steps, variance.names = 20, ["tension"]
        variance.embedding = np.ones(256, dtype=np.float32)
        result = variance.predict(["SP", "iy"], np.array([[1, 1]]), np.array([[440, 880]], dtype=np.float32))
        np.testing.assert_allclose(result["tension"], [[0.2, 0.3]])



class ProsodyTests(unittest.TestCase):
    def test_duration_prediction_preserves_each_syllable_budget(self):
        from diffsinger_onnx import allocate_durations
        result = allocate_durations([[2, 3, 20, 5, 1]], [3, 2], [50, 20])[0]
        self.assertEqual(result[:3].sum(), 50)
        self.assertEqual(result[3:].sum(), 20)
        self.assertTrue(np.all(result > 0))
        self.assertGreater(result[2], result[0] + result[1])

    def test_invalid_or_impossible_durations_fail(self):
        from diffsinger_onnx import allocate_durations
        for values, budget in [([float('nan'), 2], 10), ([0, 0], 10), ([1, 2], 1)]:
            with self.assertRaises(ValueError):
                allocate_durations(values, [2], [budget])

    def test_melisma_uses_one_syllable_with_separate_pitch_notes(self):
        from diffsinger_onnx import prosody_layout
        notes = [dict(start=0.5, duration=0.5, midi=60, lyric='stay'),
                 dict(start=1, duration=0.75, midi=64, lyric='+'),
                 dict(start=1.75, duration=0.5, midi=67, lyric='me')]
        origin, symbols, div, budgets, ph_midi, midi, dur, rest = prosody_layout(
            notes, PhonemeTests.table, 0.01, PhonemeTests.dictionary)
        self.assertEqual(symbols, ['SP', 's', 't', 'ey', 'm', 'iy', 'SP'])
        self.assertEqual(div, [1, 3, 2, 1])
        self.assertEqual(budgets, [25, 125, 50, 25])
        self.assertEqual(int(dur.sum()), sum(budgets))
        np.testing.assert_array_equal(midi, [[60, 60, 64, 67, 67]])
        np.testing.assert_array_equal(rest, [[True, False, False, False, True]])
        np.testing.assert_array_equal(ph_midi, [[60, 62, 62, 62, 67, 67, 67]])
        self.assertEqual(origin, 0.25)

    def test_nonintegral_tempo_does_not_accumulate_rounding_drift(self):
        from diffsinger_onnx import prosody_layout, allocate_durations
        notes = [dict(start=i*0.137, duration=0.137, midi=60, lyric='me') for i in range(100)]
        _, symbols, div, budgets, _, _, dur, _ = prosody_layout(
            notes, PhonemeTests.table, 512/44100, PhonemeTests.dictionary)
        predicted = allocate_durations(np.ones(len(symbols)), div, budgets)
        self.assertEqual(int(predicted.sum()), int(dur.sum()))
        self.assertTrue(np.all(predicted > 0))


class SustainedVowelTests(unittest.TestCase):
    def test_long_sail_holds_vowel_not_s_or_l(self):
        from diffsinger_onnx import allocate_durations
        frames = allocate_durations([[100, 2, 100]], [3], [200], ['s', 'ey', 'l'], .01)[0]
        self.assertEqual(frames.sum(), 200)
        self.assertLessEqual(frames[0], 7)
        self.assertLessEqual(frames[2], 7)
        self.assertGreaterEqual(frames[1], 186)

    def test_clusters_and_short_syllables_keep_positive_durations(self):
        from diffsinger_onnx import allocate_durations
        for budget in [5, 10, 50, 300]:
            frames = allocate_durations([[20, 20, 1, 20, 20]], [5], [budget],
                                        ['s', 't', 'ae', 'n', 'd'], .01)[0]
            self.assertEqual(frames.sum(), budget)
            self.assertTrue(np.all(frames > 0))
            self.assertLessEqual(frames[:2].sum(), 12)
            self.assertLessEqual(frames[3:].sum(), 12)

    def test_silence_and_language_prefix_are_preserved(self):
        from diffsinger_onnx import allocate_durations
        frames = allocate_durations([[1, 20, 1, 1]], [1, 2, 1], [25, 100, 25],
                                    ['SP', 'en/s', 'en/ey', 'SP'], .01)[0]
        self.assertEqual(frames[0], 25)
        self.assertEqual(frames[-1], 25)
        self.assertEqual(frames[1:3].sum(), 100)
        self.assertGreaterEqual(frames[2], 93)


class LanguageTests(unittest.TestCase):
    def test_language_ids_preserve_shared_silence(self):
        from diffsinger_onnx import language_ids
        result = language_ids(['SP', 'en/s', 'en/ey', 'AP', 'ja/a'], {'en': 2, 'ja': 6})
        np.testing.assert_array_equal(result, [[0, 2, 2, 0, 6]])
        self.assertEqual(result.dtype, np.int64)

    def test_unknown_language_fails(self):
        from diffsinger_onnx import language_ids
        with self.assertRaises(KeyError):
            language_ids(['en/s'], {'ja': 6})


if __name__ == "__main__":
    unittest.main()
