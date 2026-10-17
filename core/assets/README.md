# Invoice wordmark

The SVG contains only the Select Q glyph outlines from the supplied Word template,
rendered by Microsoft Word and extracted from its PDF. It retains the original
15-point sizing, #4472c4 colour, letter positions, Q outline and synthetic bold
stroke around the script. No customer information is included.

The TTF packages the complete logo as one private-use glyph (U+E000) for gxPDF.
It is drawn at 20 points to reproduce the 51 x 20 point SVG bounds. Script weight
uses overlapping outlines offset around a 24-point circle at the Word stroke's
half-width (0.214285 points). This preserves the appearance at invoice size and
keeps it scalable. No system fonts or Word installation are required at runtime.

This representation works around gxPDF v0.9.4 builder.Image rendering only a grey
placeholder. Keep the SVG as the source artwork for future native vector support.
