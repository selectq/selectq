# Bank statement download

On a statement history page, including `/history/3`, click **Download Bank
Statement** to save an Excel workbook with **Account Head**, **Sub Account Head**,
and **Invoice** after the original seven bank columns. The download includes all
statement transactions and their latest saved classifications, regardless of grid
filters or pagination. Save edits and invoice allocations before downloading.

The original workbook supplies its layout, header, footer, cell formatting, and
other sheets. The application annotates a copy; the source file is unchanged.
For Sales Invoice rows, Sub Account Head contains the business partner name, using the
linked invoices' partners if no partner is assigned directly. Invoice contains
sales invoice references or the linked purchase invoice number.

New uploads retain the original workbook in `statement_workbooks`, atomically
with the import and transactions. Older imports use the original `.xlsx` with
the recorded filename from the application folder. The original file for
statement #3 is `Acct_Statement_XXXXXXXX5472_08092026.xlsx`. Account and transaction
values must match before classifications are placed on their original rows.
Missing or mismatched originals return an error instead of a reconstructed file.

`GET /api/imports/{id}/download` returns `<original-name>-classified.xlsx`.
