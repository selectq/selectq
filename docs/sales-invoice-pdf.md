# Sales invoice PDF

Sales invoices now store optional Project Name, Our Ref, Your Ref, Order No and
Additional Information. These fields are available when creating and editing an
invoice. Line descriptions support multiple lines. Existing invoices receive empty
values through an idempotent database migration on application startup.

The invoice grid's PDF button downloads an actual PDF from
`GET /api/sales-invoices/{id}/pdf`. The Go server renders it with the existing
gxPDF v0.9.4 dependency; no print dialog or Word installation is needed.

Select invoice rows and click **Download selected as ZIP** to download up to 100
invoices together. The header checkbox selects filtered invoices across pages.
Each ZIP entry is a separate invoice PDF with its annexure. The bulk endpoint is
`POST /api/sales-invoices/pdf-download` with `{"ids":[1,2]}`; invalid or missing
invoices fail the download without returning a partial archive.

The layout follows the information in the supplied Studio South Canopy Houston
document. The first page includes the selected contact, saved billing address,
saved seller GSTIN, buyer GSTIN, project/references, dates, item table and totals.
The heading displays `Invoice: <Invoice Number>` using the saved invoice number.
The seller ("From") block starts farther right, using a 7:5 buyer/seller column split.
INR invoices retain the existing CGST/SGST or IGST calculation; other currencies
have no GST columns. Payment currency and payer transfer-charge instructions print
below the table, followed by optional invoice notes and the annexure reference.

The bank annexure is fixed in `core/sales_pdf.go`, using the beneficiary and HDFC
details supplied in the Word document. It is independent of invoice values and
company-profile edits. The annexure uses a bordered two-column table, with field
labels on the left and account/bank details on the right, including multiline
addresses and separate beneficiary and bank section headings. Its borders use the
same subtle light grey (`#cccccc`) and 0.5-point width as the first-page item table.
Normal invoices have two pages. Longer invoices paginate
before the annexure, which remains the final standalone page.

The layout is recreated in PDF, rather than converting the Word document with
identical typography. [Review the generated sample](sales-invoice-logo-sample.pdf).
The sample uses July 11 as its due date: July 1 plus 10 calendar days under the
existing application rule. The Word sample says July 10, which is inconsistent
with that rule.

Validation covers new-field create/update/list persistence, saved address and GST
data, USD and INR totals, direct HTTP download and errors, two-page output, and
60-line overflow without lost rows or changes to the annexure.

The template letterhead repeats on every page, including the bank annexure and
continuation pages: blue Select Q wordmark, registered-office line, blue rules,
GSTIN, email and the complete registered address. These are fixed template values.
The wordmark uses vector outlines extracted from Word's rendered template,
preserving its heavier script, Q shape, spacing and blue colour. The source SVG
is retained in `core/assets`; a single embedded logo glyph supports rendering
with gxPDF v0.9.4 without system-font substitution.
