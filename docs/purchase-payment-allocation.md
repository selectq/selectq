# Purchase payment allocation

Click the Invoices cell of an eligible bank withdrawal to open purchase allocations.
Register any missing invoices, then enter the INR amount paid against each invoice
and choose **Save allocations**. For a withdrawal of INR 4,567, allocations may be
INR 2,000, INR 1,500 and INR 1,067 across three invoices.

One withdrawal can pay several invoices, and one invoice can receive payments from
several withdrawals. Each allocation must have at most two decimal places. The
sum cannot exceed the withdrawal or the respective invoice's outstanding balance.
Unused payment amounts remain unallocated. Set an allocation to zero and save to
remove a link. Saves replace all allocations for that withdrawal atomically.

The register shows paid and outstanding INR amounts. Statement downloads include
all linked purchase invoice references. Uploaded documents remain attached to
their individual invoices. Salary, personal, sales-receipt and deposit transactions
cannot receive purchase allocations. Remove allocations before changing a payment
to an incompatible classification. Invoice totals cannot be reduced below payments.

Existing single-invoice links migrate automatically on startup. Since those links
did not record an allocated amount, the initial allocation is the lesser of the
invoice total and withdrawal amount. Review and adjust these amounts where needed.
Existing invoice IDs, documents and metadata are preserved. New invoices created
from a withdrawal are registered first; save their payment allocations separately.

`PUT /api/transactions/{id}/purchase-allocations` accepts:

```json
{"allocations":[{"purchase_invoice_id":1,"amount":2000},{"purchase_invoice_id":2,"amount":2567}]}
```

An empty array removes all links. Omitting the array is rejected. Invoice responses
include `payments`, `paid_amount` and `outstanding_amount`. The older single-link
endpoint remains available for existing clients.
