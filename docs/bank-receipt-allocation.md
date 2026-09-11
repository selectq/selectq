# Bank receipt allocation

Click the Invoices cell on a statement transaction, select a business partner, and enter the amount allocated to each invoice. Save allocations applies the
classification and links together, or rejects the entire change.

The invoice allocation table is the receipt subledger: one bank payment can
credit multiple invoices and an invoice can receive multiple payments. Amounts
are in invoice currency. Any unallocated receipt remains available on that bank
transaction. This does not introduce a general-ledger journal system.

INR invoice amounts in this application are stored before GST. Expected cash is
90% of that amount, implementing the requested GST exclusion and fixed 10% TDS.
Thus a 100 base plus 18 GST invoice expects 90 cash. Foreign invoices use their
full amount, and allocations require a matching bank forex currency and amount.
No tolerance or shortfall write-off is applied. A balance of 0.50 remains open.

Edit an invoice and check Closed to prevent further allocations. Existing
allocations remain valid. Closed and fully paid invoices are excluded from new
suggestions; current links remain visible. Linked invoice financial details and
partner cannot be changed, preventing allocations from becoming inconsistent.

The database migration runs on application startup and preserves existing data.
Old free-text invoice references remain visible but are not silently converted
into financial allocations; allocate those receipts explicitly in the UI.

PUT /api/transactions/{id} accepts allocations:

```json
{
  "account_head": "Sales Invoice",
  "sub_account_head": "",
  "business_partner_id": 1,
  "allocations": [
    {"sales_invoice_id": 1, "amount": 90},
    {"sales_invoice_id": 2, "amount": 60}
  ]
}
```

Omitting allocations preserves current links; an empty array removes them.
Invoice responses expose is_closed, expected_receipt, received_amount,
outstanding_amount and is_settled. Monetary allocations have two decimal places.

## Entity relationship diagrams

See [Bank statement classification ERDs](bank-statement-classification-erd.md)
for standalone SVG diagrams of invoice setup, receipt allocation and statement
imports, with field definitions, cardinalities and worked payment examples.
