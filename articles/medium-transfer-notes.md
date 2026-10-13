# Transferring the article to Medium

Article: [Make Music With Maths and Go (III)](make-music-with-maths-and-go-iii.md)

## What is ready

- The full article uses one title, an italic subtitle, ordinary headings, paragraphs, links and short lists.
- Two equations are written as ordinary text. No LaTeX renderer or equation images are required.
- The Go snippets are excerpts from the implementation, reformatted for reading. The kick function splits one expression into named variables without changing its behaviour. They depend on the surrounding Muse package; they are not standalone programs.
- Sources are linked beside the relevant explanations.
- There are no Markdown tables, footnotes or local file links in the article body.

## Transfer checklist

1. Open the article in a Markdown preview and copy the rendered content into a Medium draft. Treat this as a starting point: paste behaviour can vary. Pasting raw Markdown may leave its punctuation visible.
2. Set the title and subtitle, then check the section headings and subheadings.
3. Check the code blocks separately. If paste does not preserve them, create code blocks in the destination editor and paste the code from the Markdown source. Preserve indentation and ordinary straight quotes. Do not include the Markdown fence lines or language labels in the code itself.
4. Keep the equations as text. They read `A(n) = sin(n × π × p) / n` and `f(t) = 48 + 100 × exp(-t / 0.035)`. The prose defines their symbols, including exp.
5. Check that the inline source links survived the transfer.
6. Replace the final audio marker with a public player or a public audio link that works in the destination. The local WAV cannot become a public embed simply by pasting its filesystem path. No audio has been uploaded or published.
7. Before publishing, confirm that the public repository contains the features and `scores/bright_side.yaml` described in the article. The article was checked against the local working tree, which includes uncommitted changes.
8. Preview the draft, especially the long Go snippet, on a narrow screen. Remove the audio marker after inserting the actual player or link.

## Closing sample

- Audio: [bright-side.wav](assets/bright-side.wav)
- Score snapshot used for that render: [bright-side.yaml](assets/bright-side.yaml)
- Rendered sample: 62.760 seconds, stereo, 44.1 kHz, signed 16-bit PCM; zero samples at either clipping limit in the checked output.
- The audio was generated and checked during the preceding article work. It has been copied here unchanged so the article does not depend on a temporary directory. The score snapshot is the corresponding rendering input.

## Attribution and editorial notes

The code explicitly credits Freeverb and the RBJ biquad design. Other references support the explanations of established techniques; they are not evidence that those sources were consulted during development. Formant-frequency correspondence with the Csound tenor table is a checked numerical match, not a claim about implementation history.

This article covers the algorithmic vowel instrument, not the separate experimental lyric-singing backend. Sheet-music conversion is intentionally outside its scope.

The source articles on Medium were not fully accessible during drafting. The tone follows the agreed conversational, educational revision, rather than a verified line-by-line style comparison with the complete earlier articles.
