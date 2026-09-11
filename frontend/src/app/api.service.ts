import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';

export interface ImportRecord {
  id: number;
  account_no: string;
  source_file: string;
  statement_from: string;
  statement_to: string;
  imported_at: string;
}

export interface InvoiceAllocation { sales_invoice_id: number; amount: number; }

export interface BankTransaction {
  allocations?: InvoiceAllocation[];
  id: number;
  date: string;
  narration: string;
  chq_ref_no: string;
  value_date: string;
  withdrawal_amt: number;
  deposit_amt: number;
  closing_balance: number;
  account_head: string;
  sub_account_head: string;
  invoice_number: string;
  business_partner_id?: number;
  business_partner_name?: string;
  currency: string;
  exchange_rate: number;
  forex_amount: number;
}

export interface BusinessPartnerContact {
  id?: number;
  business_partner_id?: number;
  name: string;
  email: string;
  phone: string;
  is_primary: boolean;
}

export interface BPAddress {
  gstin?: string;
  id?: number;
  business_partner_id?: number;
  address: string;
  previous_address_id?: number | null;
  is_archived: boolean;
  created_at?: string;
  was_archived?: boolean;
}

export interface BusinessPartner {
  addresses?: BPAddress[];
  id?: number;
  name: string;
  billing_address: string;
  invoice_currency: string;
  tax_information: string;
  contacts?: BusinessPartnerContact[];
}

export interface InvoiceLineItem {
  cgst_percent?: number;
  sgst_percent?: number;
  igst_percent?: number;
  id?: number;
  sales_invoice_id?: number;
  description: string;
  hsn_sac_code: string;
  quantity: number;
  rate: number;
  gst_percent: number;
  amount: number;
}

export interface SalesInvoice {
  seller_gstin?: string;
  buyer_gstin?: string;
  gst_treatment?: 'intrastate' | 'interstate' | 'none';
  gst_amount?: number;
  cgst_amount?: number;
  sgst_amount?: number;
  igst_amount?: number;
  address_id?: number | null;
  billing_address?: string;
  is_closed?: boolean;
  expected_receipt?: number;
  received_amount?: number;
  outstanding_amount?: number;
  is_settled?: boolean;
  id?: number;
  invoice_number: string;
  financial_year: string;
  business_partner_id: number;
  business_partner_name?: string;
  contact_id?: number;
  invoice_date: string;
  due_in_days?: number;
  due_date?: string;
  currency: string;
  amount: number;
  line_items?: InvoiceLineItem[];
}

export interface CompanyProfile {
  id?: number;
  company_name: string;
  address: string;
  gstin: string;
  pan: string;
  email: string;
  phone: string;
}

export interface DBResult { columns: string[]; rows: unknown[][]; truncated: boolean; changes: number; }
export interface DBObject { type: string; name: string; table_name: string; sql: string; }
export interface DBObjectDetail { object: DBObject; sections: Record<string, DBResult>; }

export interface GSTR3BRow {
  invoice_number: string; currency: string; amount: number; invoiced_to: string;
  address: string; invoice_date: string; exchange_rate: number | null;
  value_inr: number | null; reference_date: string | null;
}
export interface GSTR3BSummary { financial_year: string; financial_years: string[]; rows: GSTR3BRow[]; }

export interface ReferenceRate {
  rate_date: string;
  currency: string;
  units: number;
  rate_inr: number;
  source_file?: string;
  source_sheet?: string;
  imported_at?: string;
}
export interface ReferenceRateImport { empty_skipped: number; sheet: string; inserted: number; skipped: number; from_date: string; to_date: string; }

@Injectable({
  providedIn: 'root'
})
export class ApiService {
  private baseUrl = '/api';

  constructor(private http: HttpClient) {}

  getGSTR3B(financialYear = ''): Observable<GSTR3BSummary> {
    return this.http.get<GSTR3BSummary>(`${this.baseUrl}/gstr3b`, { params: { financial_year: financialYear } });
  }
  downloadGSTR3B(financialYear: string): Observable<Blob> {
    return this.http.get(`${this.baseUrl}/gstr3b/download`, { params: { financial_year: financialYear }, responseType: 'blob' });
  }

  getDBObjects(): Observable<DBObject[]> { return this.http.get<DBObject[]>(`${this.baseUrl}/db-browser/objects`); }
  getDBDetail(name: string): Observable<DBObjectDetail> { return this.http.get<DBObjectDetail>(`${this.baseUrl}/db-browser/detail`, { params: { name } }); }
  getDBRows(name: string, offset: number, limit = 100): Observable<DBResult> { return this.http.get<DBResult>(`${this.baseUrl}/db-browser/rows`, { params: { name, offset, limit } }); }
  getDBSettings(): Observable<Record<string, DBResult>> { return this.http.get<Record<string, DBResult>>(`${this.baseUrl}/db-browser/settings`); }
  executeSQL(sql: string, mode: string): Observable<DBResult> { return this.http.post<DBResult>(`${this.baseUrl}/db-browser/sql`, { sql, mode }); }

  getReferenceRates(): Observable<ReferenceRate[]> {
    return this.http.get<ReferenceRate[]>(`${this.baseUrl}/reference-rates`);
  }
  createReferenceRate(rate: ReferenceRate): Observable<ReferenceRate> {
    return this.http.post<ReferenceRate>(`${this.baseUrl}/reference-rates`, rate);
  }
  updateReferenceRate(original: ReferenceRate, rate: ReferenceRate): Observable<ReferenceRate> {
    return this.http.put<ReferenceRate>(`${this.baseUrl}/reference-rates/${encodeURIComponent(original.rate_date)}/${encodeURIComponent(original.currency)}`, rate);
  }
  uploadReferenceRates(file: File, sheet: string): Observable<ReferenceRateImport> {
    const data = new FormData();
    data.append('file', file);
    if (sheet.trim()) data.append('sheet', sheet.trim());
    return this.http.post<ReferenceRateImport>(`${this.baseUrl}/reference-rates/import`, data);
  }

  uploadStatement(file: File): Observable<any> {
    const formData = new FormData();
    formData.append('file', file);
    return this.http.post(`${this.baseUrl}/upload`, formData);
  }

  getImports(): Observable<ImportRecord[]> {
    return this.http.get<ImportRecord[]>(`${this.baseUrl}/imports`);
  }

  getTransactions(importId: number): Observable<BankTransaction[]> {
    return this.http.get<BankTransaction[]>(`${this.baseUrl}/imports/${importId}/transactions`);
  }

  updateTransaction(txnId: number, payload: any): Observable<any> {
    return this.http.put(`${this.baseUrl}/transactions/${txnId}`, payload);
  }

  getBusinessPartners(query?: string): Observable<BusinessPartner[]> {
    let url = `${this.baseUrl}/business-partners`;
    if (query) {
      url += `?q=${encodeURIComponent(query)}`;
    }
    return this.http.get<BusinessPartner[]>(url);
  }

  createBusinessPartner(partner: BusinessPartner): Observable<BusinessPartner> {
    return this.http.post<BusinessPartner>(`${this.baseUrl}/business-partners`, partner);
  }

  getBusinessPartner(id: number): Observable<BusinessPartner> {
    return this.http.get<BusinessPartner>(`${this.baseUrl}/business-partners/${id}`);
  }
  
  updateBusinessPartner(id: number, partner: BusinessPartner): Observable<BusinessPartner> {
    return this.http.put<BusinessPartner>(`${this.baseUrl}/business-partners/${id}`, partner);
  }

  deleteBusinessPartner(id: number): Observable<any> {
    return this.http.delete(`${this.baseUrl}/business-partners/${id}`);
  }

  getSalesInvoices(): Observable<SalesInvoice[]> {
    return this.http.get<SalesInvoice[]>(`${this.baseUrl}/sales-invoices`);
  }

  getSalesInvoice(id: number): Observable<SalesInvoice> {
    return this.http.get<SalesInvoice>(`${this.baseUrl}/sales-invoices/${id}`);
  }

  createSalesInvoice(invoice: SalesInvoice): Observable<SalesInvoice> {
    return this.http.post<SalesInvoice>(`${this.baseUrl}/sales-invoices`, invoice);
  }

  updateSalesInvoice(id: number, invoice: SalesInvoice): Observable<SalesInvoice> {
    return this.http.put<SalesInvoice>(`${this.baseUrl}/sales-invoices/${id}`, invoice);
  }

  getCompanyProfile(): Observable<CompanyProfile> {
    return this.http.get<CompanyProfile>(`${this.baseUrl}/company-profile`);
  }

  saveCompanyProfile(profile: CompanyProfile): Observable<CompanyProfile> {
    return this.http.put<CompanyProfile>(`${this.baseUrl}/company-profile`, profile);
  }
}
