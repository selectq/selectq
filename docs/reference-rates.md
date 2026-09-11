# INR reference rates

The supplied `ReferenceRate.xlsx`, worksheet `BankWise`, is imported into the
separate `reference_rates` table. It contains 102 dates from 2 April to 31 August
2026 and six currencies (USD, GBP, EUR, JPY, AED, IDR): 612 rate records.

## Main application

Open **Reference Rates** using the table/upload SVG icon in the sidebar.

- Choose an XLSX file and click **Import rates**. The worksheet is detected
  automatically from its first-row headers, regardless of its name or position. The UI accepts files up to 9 MB; the API caps the complete
  multipart request at 10 MB. The import summary shows the detected sheet, inserted rates, identical
  rates skipped, blank currency cells skipped, and the scanned date range, and the grid refreshes after success.
- Browse, filter, sort and paginate saved rates in AG Grid. Quote units are shown
  beside the INR rate so JPY and IDR quotes remain unambiguous.
- Click **Add rate** for manual entry, or **Edit** on a row. Date, currency, units
  and rate can all be corrected. New JPY/IDR entries default to 100/10,000 units;
  the units remain editable. Save validates calendar dates, foreign currency
  codes, positive whole quote units and positive finite rates.
- One rate is allowed per date/currency. Conflicting creates or edits fail without
  overwriting another entry. A failed save retains the user's form values.
- Manual saves label the source `Manual entry`, clear the worksheet and set the
  saved timestamp to UTC now. This identifies the latest entry source; this table
  does not keep a revision history. Reimporting a workbook with a different quote
  for a manually edited date/currency reports a conflict.

## Table

| Column | Meaning |
| --- | --- |
| `rate_date` | Quote date, stored as `YYYY-MM-DD` |
| `currency` | Foreign currency code |
| `units` | Number of foreign currency units covered by the quote |
| `rate_inr` | INR quoted for those units, without import-time rounding |
| `source_file` | Workbook filename or `Manual entry` |
| `source_sheet` | Worksheet name |
| `imported_at` | UTC import or latest manual-save timestamp |

The primary key is `(rate_date, currency)`. The table is independent of invoices
and transactions. It is automatically available in the existing **DB Browser**.
Startup creates the table if absent; it does not automatically import a workbook.

USD, GBP, EUR and AED quotes cover one unit. JPY quotes cover **100 JPY** and IDR
quotes cover **10,000 IDR**. Preserve these units when implementing conversion:

```text
INR amount = foreign amount * rate_inr / units
```

For example, the 31 August 2026 JPY quote stores `units=100` and
`rate_inr=59.7200`, rather than treating 59.7200 as the rate for one yen.
Rates use SQLite REAL storage, consistent with existing application amounts;
future conversion logic should specify its monetary rounding explicitly.

## API

| Method | Endpoint | Purpose |
| --- | --- | --- |
| GET | `/api/reference-rates` | List all rates by date descending, then currency |
| POST | `/api/reference-rates/import` | Multipart upload: `file` and optional `sheet` |
| POST | `/api/reference-rates` | Create a manual rate |
| PUT | `/api/reference-rates/{date}/{currency}` | Edit the original date/currency entry |

Manual request example:

```json
{"rate_date":"2026-08-31","currency":"JPY","units":100,"rate_inr":59.72}
```

The PUT URL identifies the original entry; the body can change its date or
currency. Responses return the saved row. Invalid input returns 400, a missing
edit target 404, and a duplicate manual entry 409. Source fields in manual
requests are ignored and set by the server.

## Repeatable command-line import

Run from the repository root:

```powershell
go run ./cmd/import-reference-rates -db bank_statements.db -file ReferenceRate.xlsx
```

The command reports `inserted`, `skipped` (identical existing quotes),
`empty_skipped` (blank currency cells), `sheet` (selected worksheet), and
`from_date` / `to_date` (the range of valid dated rows scanned) as JSON.
The expected workbook header is `Date`, followed by columns such as
`USD (INR / 1 USD)` and `JPY (INR / 100 JPY)`. Currency columns can be reordered.
Dates may be day/month/year text, ISO dates, or Excel serial dates using the
workbook's date system. Entirely blank rows are ignored.

When no worksheet override is provided, the importer inspects each worksheet's
first row for `Date` and valid currency headers. Unrelated cover sheets are
ignored. Exactly one matching worksheet is selected automatically. No matching
sheet, or several matching sheets, produces an error without importing data.
For several matches, enter one of the names listed in the error in the UI's
**Worksheet override (optional)** field, pass `sheet` in the multipart request,
or use `-sheet "Worksheet Name"` in the command. Choosing another file in the UI
resets the override to automatic detection.

A blank, missing, or whitespace-only currency cell is skipped without inserting
zero, deleting an existing rate, or skipping other currencies on that date.
Missing trailing cells are handled the same way. A dated row with all currency
cells blank is accepted and contributes to `empty_skipped`. A sheet containing
only such dated rows returns success with zero inserted rates. Text such as
`N/A` or `null` is not an empty Excel cell and remains invalid.

The complete import runs in one transaction. Invalid headers, dates, nonempty invalid or
nonpositive rates, and conflicting existing quotes abort the import. Reimporting
identical date/currency/units/rate combinations skips them without changing their
original provenance or timestamps. A different quote for an existing date and
currency is rejected for review rather than overwriting it.

## Inspection

In DB Browser, select `reference_rates`, or execute:

```sql
SELECT rate_date, currency, units, rate_inr
FROM reference_rates
ORDER BY rate_date, currency;
```

Date selection, weekend/holiday fallback, invoice conversion and final INR
rounding remain for the forthcoming conversion requirements. This import stores
only the dates and quotes present in the supplied workbook.

Go tests cover quote units, source metadata, date parsing, repeated imports,
sparse and all-blank currency data, worksheet discovery and overrides,
invalid data rollback, multipart upload, manual create/edit, and preservation of
existing rates on conflicts. Angular tests cover the page, manual unit defaults,
save conflicts, upload summaries, optional worksheet overrides and retaining
input after failures.
