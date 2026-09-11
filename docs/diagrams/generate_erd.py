"""Regenerate the standalone SVG ER diagrams using only Python's standard library."""
from pathlib import Path
from html import escape
import xml.etree.ElementTree as ET

OUT = Path(__file__).resolve().parent

def diagram(filename, title, subtitle, height, cards, edges, notes):
    parts = [f'''<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="{height}" viewBox="0 0 1200 {height}" role="img" aria-labelledby="title desc">
<title id="title">{escape(title)}</title><desc id="desc">{escape(subtitle)} Relationships show cardinality at each end. PK means primary key; FK means foreign key; UQ means unique.</desc>
<style>text{{font-family:Segoe UI,Arial,sans-serif;fill:#172b4d}} .title{{font-size:28px;font-weight:700}} .sub{{font-size:15px;fill:#52657c}} .entity{{font-size:19px;font-weight:600;fill:white}} .field{{font-family:Consolas,monospace;font-size:15px}} .label{{font-size:14px;fill:#254e70;paint-order:stroke;stroke:#f4f7fb;stroke-width:6px;stroke-linejoin:round}} .note{{font-size:15px;fill:#354a62}}</style>
<rect width="1200" height="{height}" fill="#f4f7fb"/>
<text x="40" y="48" class="title">{escape(title)}</text><text x="40" y="78" class="sub">{escape(subtitle)}</text>''']
    for path, labels in edges:
        parts.append(f'<path d="{path}" fill="none" stroke="#66849d" stroke-width="2"/>')
        for x,y,text in labels:
            parts.append(f'<text x="{x}" y="{y}" text-anchor="middle" class="label">{escape(text)}</text>')
    for x,y,w,name,fields,color in cards:
        h=54+len(fields)*27
        parts.extend([f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="10" fill="white" stroke="#c6d4e1"/>',f'<path d="M{x+10} {y} H{x+w-10} Q{x+w} {y} {x+w} {y+10} V{y+42} H{x} V{y+10} Q{x} {y} {x+10} {y}" fill="{color}"/>',f'<text x="{x+16}" y="{y+28}" class="entity">{escape(name)}</text>'])
        for i,field in enumerate(fields):
            parts.append(f'<text x="{x+16}" y="{y+68+i*27}" class="field">{escape(field)}</text>')
    for x,y,line in notes:
        parts.append(f'<text x="{x}" y="{y}" class="note">{escape(line)}</text>')
    parts.append(f'<text x="40" y="{height-24}" class="sub">PK primary key · FK foreign key · UQ unique · 1 exactly one · 0..1 optional · 0..* zero or many</text></svg>')
    svg='\n'.join(parts)
    ET.fromstring(svg)
    (OUT/filename).write_text(svg,encoding='utf-8')

blue='#215b88'; teal='#087e83'; purple='#6952a3'
diagram('invoice-setup.svg','01 / Create an invoice for a business partner','Invoice ownership, optional contact and item-level GST. Selected columns shown.',900,[
 (50,130,350,'business_partners',['PK id','UQ name','invoice_currency','billing_address'],blue),
 (800,130,350,'business_partner_contacts',['PK id','FK business_partner_id','name / email / phone','is_primary'],blue),
 (50,450,390,'sales_invoices',['PK id','UQ invoice_number','FK business_partner_id','FK contact_id (nullable)','financial_year / invoice_date','currency / amount (before GST)','is_closed'],teal),
 (800,450,350,'sales_invoice_line_items',['PK id','FK sales_invoice_id','description / hsn_sac_code','quantity / rate','gst_percent / amount'],teal)
],[
 ('M400 205 H800',[(425,194,'1'),(775,194,'0..*'),(600,194,'has contacts')]),
 ('M225 292 V450',[(240,319,'1'),(245,429,'0..*'),(310,375,'owns invoices')]),
 ('M975 292 V375 H600 V505 H440',[(992,320,'0..1'),(469,494,'0..*'),(720,364,'optional invoice contact')]),
 ('M440 610 H800',[(466,599,'1'),(772,599,'0..*'),(620,599,'contains')])
],[(50,770,'Each invoice has one partner; a contact may be omitted. Each line item belongs to one invoice.'),
   (50,797,'amount on an invoice is the sum of stored line-item amounts before GST.'),
   (50,824,'Numbering uses invoice_sequences; seller details use company_profile. Neither has an invoice foreign key.')])

diagram('receipt-allocation.svg','02 / Allocate bank receipts to sales invoices','Many-to-many payments through invoice_allocations; all selected invoices must share the receipt partner.',1060,[
 (425,125,350,'business_partners',['PK id','UQ name'],blue),
 (50,340,420,'bank_transactions',['PK id','FK business_partner_id (nullable)','deposit_amt / withdrawal_amt','currency / forex_amount','exchange_rate','account_head / sub_account_head','invoice_number (legacy text)'],blue),
 (730,340,420,'sales_invoices',['PK id','FK business_partner_id','UQ invoice_number','currency','amount (before GST)','is_closed'],teal),
 (390,775,420,'invoice_allocations',['PK, FK bank_transaction_id','PK, FK sales_invoice_id','amount (invoice currency)'],purple)
],[
 ('M425 210 H260 V340',[(395,199,'0..1'),(282,326,'0..*'),(265,247,'receipt partner')]),
 ('M775 210 H940 V340',[(801,199,'1'),(964,326,'0..*'),(942,247,'invoice owner')]),
 ('M260 583 V710 H460 V775',[(276,618,'1'),(483,762,'0..*'),(294,698,'allocates receipt')]),
 ('M940 556 V710 H740 V775',[(958,591,'1'),(765,762,'0..*'),(926,698,'receives payment')])
],[(50,948,'The two FK columns together form the primary key: one row per receipt / invoice pair.'),
   (50,975,'Matching partners, currencies, available balances and closed status are checked by application code.'),
   (50,1002,'A receipt can leave cash unallocated. An invoice can remain partially paid across multiple receipts.')])

diagram('statement-import.svg','03 / Trace a receipt to its bank statement','Bank account and import provenance for transactions later allocated to invoices.',820,[
 (50,130,350,'accounts',['PK account_no','customer_id','branch / ifsc / micr'],blue),
 (800,130,350,'imports',['PK id','FK account_no','source_file','statement_from / statement_to','imported_at'],blue),
 (390,460,420,'bank_transactions',['PK id','FK import_id','FK account_no','txn_date / narration','deposit_amt / withdrawal_amt','closing_balance'],teal)
],[
 ('M400 190 H800',[(425,179,'1'),(775,179,'0..*'),(600,179,'has statement imports')]),
 ('M225 265 V385 H450 V460',[(240,290,'1'),(475,446,'0..*'),(253,374,'has transactions')]),
 ('M975 319 V385 H750 V460',[(990,344,'1'),(775,446,'0..*'),(949,374,'contains rows')])
],[(50,728,'Each transaction references one import and one account; the import also references one account.'),
   (50,755,'The importer writes matching account numbers. The schema has no composite FK enforcing that match.')])

print('Generated and XML-validated 3 SVG diagrams.')
