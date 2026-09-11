# Invoice CGST, SGST and IGST

The invoice form applies the state comparison and missing-GSTIN fallback requested
in `raisingNewInvoice.md`. Buyer registration comes from the **selected billing
address**, not the business partner's free-text tax information.

## Setup and use

Set the seller's GSTIN under **Settings / Company Profile**. On each business
partner address, enter that location's GSTIN, or leave it blank. GSTIN is trimmed
and converted to uppercase on save. A nonempty value must contain 15 alphanumeric
characters and begin with two digits; this checks the format only, not registration
status or the GSTIN checksum.

Choose the partner and billing address when raising an invoice. At the default
18% total GST rate, the form selects:

| Invoice / GSTIN condition | CGST | SGST | IGST |
| --- | ---: | ---: | ---: |
| INR, seller and buyer GSTIN state prefixes match | 9% | 9% | 0% (disabled) |
| INR, seller and buyer GSTIN state prefixes differ | 0% (disabled) | 0% (disabled) | 18% |
| INR, either GSTIN is missing | 9% | 9% | 0% (disabled) |
| Non-INR | 0% | 0% | 0% |

Choose **Total GST %** to change a line's overall rate. For an intrastate invoice,
the rate is divided equally between CGST and SGST; for an interstate invoice,
the full rate is assigned to IGST. The component fields are calculated and read-only;
inapplicable fields are also disabled. The totals and printable invoice show
the active components separately. Changing the selected address updates the split.

The first two GSTIN characters identify the registration state, as described in
[CBIC's registration rules](https://cbic-gst.gov.in/gst-registration-rules.html).
The configured missing-GSTIN fallback is the application's requested default.
This feature uses the requested GSTIN-prefix comparison; it does not add separate
place-of-supply, SEZ, or other special-treatment overrides.

## Amounts and rounding

CGST + SGST + IGST equals the total GST; splitting it does not add tax twice.
The existing receipt rule remains:

```text
Expected INR receipt = pre-GST base - 10% TDS on that base + total GST
Outstanding = expected receipt - allocated receipts
```

For base 209,000 and 18% GST, total GST is 37,620. A same-state invoice shows
18,810 CGST and 18,810 SGST; an interstate invoice shows 37,620 IGST. Both expect
225,720 cash before any receipts are allocated.

Invoice GST totals are rounded to two decimals. Intrastate CGST is the rounded
half of the total; SGST receives the remaining paise so the two always add up
to that total. Line percentages preserve the exact half-rate, such as 2.5% each
when total GST is 5%. Non-INR invoices continue to have no GST in this application.

## Storage and history

- `bp_addresses.gstin` belongs to an immutable address version. Editing only the
  GSTIN still inserts a new version and archives its predecessor. Old invoices
  retain the GSTIN associated with their selected address ID.
- `sales_invoices.seller_gstin` snapshots the seller GSTIN at invoice creation.
  Changing the company profile does not alter already saved invoices.
- `sales_invoices.gst_treatment` stores `intrastate`, `interstate`, or `none`.
  The server determines it from the invoice currency, saved seller GSTIN and
  selected address GSTIN. On invoice edits, selecting a different address
  recalculates treatment using that invoice's original seller GSTIN.
- `sales_invoice_line_items.gst_percent` remains the persisted total rate.
  Component percentages are derived rather than stored independently, preventing
  inconsistent totals. The server ignores client-supplied component percentages
  and tax-context fields and calculates the authoritative values.

Invoice API responses include `seller_gstin`, `buyer_gstin`, `gst_treatment`,
`gst_amount`, `cgst_amount`, `sgst_amount`, and `igst_amount`. Each line includes
`cgst_percent`, `sgst_percent`, and `igst_percent`. Address payloads and responses
include `gstin`; send the current GSTIN along with address text when editing or
archiving a saved address. An empty GSTIN on an active address removes it by
creating a new version.

The startup migration adds these columns and snapshots the seller information
available for existing invoices. Existing address GSTINs start blank; the
migration does not guess them from `business_partners.tax_information`. Old INR
invoices therefore use the missing-GSTIN fallback until their selected address
is explicitly changed. Repeated startup preserves snapshots and versions.

## Verification and references

Go tests cover matching/different states, missing GSTINs, non-INR handling,
normalization, invalid input, GSTIN-only address versions, database immutability,
client attempts to override calculated fields, and repeatable migration. UI tests
verify component values and disabled states, address changes, and saved-invoice
PDF output.

Implementation: [tax rules](../core/gst_tax.go),
[address versions](../core/bp_addresses.go),
[invoice editor](../frontend/src/app/sales-invoices/list/list.component.ts).
See the [updated ERDs](bank-statement-classification-erd.md) for the added fields.
