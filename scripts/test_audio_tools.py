import array
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
import wave

from audio_report import inspect


class AudioToolsTest(unittest.TestCase):
    def write(self, path, data):
        samples = array.array('h', data)
        if sys.byteorder != 'little':
            samples.byteswap()
        with wave.open(str(path), 'wb') as wav:
            wav.setparams((1, 2, 44100, 0, 'NONE', 'not compressed'))
            wav.writeframes(samples.tobytes())

    def test_matching_and_preservation(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            a, b, out = root/'a.wav', root/'b.wav', root/'matched'
            # An interior transient forces reduction of the shared RMS target.
            signal = [100 if i % 2 else -100 for i in range(4410)]
            signal[2000] = 20000
            self.write(a, signal)
            self.write(b, [x//2 for x in signal])
            original = a.read_bytes()
            command = [sys.executable, str(Path(__file__).with_name('match_audio.py')), str(a), str(b), '--out', str(out), '--rms', '-12']
            result = subprocess.run(command, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            x, y = inspect(out/'before.wav'), inspect(out/'after.wav')
            self.assertEqual(x['clipped_samples'] + y['clipped_samples'], 0)
            self.assertAlmostEqual(x['rms_dbfs'], y['rms_dbfs'], places=3)
            self.assertLessEqual(x['peak_dbfs'], -.99)
            self.assertLess(x['rms_dbfs'], -12)
            self.assertEqual(a.read_bytes(), original)
            self.assertNotEqual(subprocess.run(command, capture_output=True).returncode, 0)
            self.assertIn('reports', json.loads((out/'comparison.json').read_text()))

    def test_report_detects_clipping(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp)/'clipped.wav'
            self.write(path, [0, 32767, -32768, 0])
            self.assertEqual(inspect(path)['clipped_samples'], 2)


if __name__ == '__main__':
    unittest.main()
