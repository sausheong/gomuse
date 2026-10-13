# Editable-score conversion reference

`expanded.yaml` and `expanded.wav` preserve Misty before the authoring change.
`compact.yaml` is the frozen compact equivalent used by the regression test.
The working, human-editable score remains `scores/misty_final.yaml` at the repo root.
Editing that working score does not violate the frozen equivalence test.

The conversion reduced the working YAML from 1,929 lines to 254. Notes, detailed
expression and performance controls expand equivalently; the fresh rendered WAV
is byte-identical to `expanded.wav`. `comparison.json` records hashes and counts;
`audio-report.json` records duration, format, peak, RMS and clipping.

Verification: all-package tests passed with the expensive all-sample rendering
batch excluded. Targeted authoring race tests, go vet and diff checks passed.
Misty was rendered separately in full and compared byte for byte. Tests cover
insertion-safe marks and phrase boundaries, custom styles, explicit-zero overrides,
source immutability and rejecting conflicting or invalid syntax.
