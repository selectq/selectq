# Sales invoice grid

The **View Invoices** grid starts with these columns:

1. Invoice number, sorted ascending by default.
2. Currency.
3. Amount (the existing pre-GST invoice amount).
4. Invoice date.
5. Business partner name and billing address.

Financial year, due date, closed status, receipt balances, item count, and edit/PDF
actions follow these columns. Users can still change the sort through the headers.
Invoice numbers use numeric-aware sorting, so sequence 999 precedes 1000.

The partner name and address appear on separate lines. Existing address line breaks
are preserved, long lines wrap, and row heights expand to show the full address.
The address comes from the invoice's saved address version, including archived
versions; changing the partner's current address does not change this display.
Invoices without a billing address show just the partner name. The column's filter
searches both the name and address.

Source requirement: [salesInvoiceDisplay.md](../salesInvoiceDisplay.md).
