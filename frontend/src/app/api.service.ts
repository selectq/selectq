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

export interface BankTransaction {
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

export interface BusinessPartner {
  id?: number;
  name: string;
  billing_address: string;
  invoice_currency: string;
  tax_information: string;
}

export interface SalesInvoice {
  id?: number;
  invoice_number: string;
  financial_year: string;
  business_partner_id: number;
  business_partner_name?: string;
  invoice_date: string;
  currency: string;
  amount: number;
}

@Injectable({
  providedIn: 'root'
})
export class ApiService {
  private baseUrl = '/api';

  constructor(private http: HttpClient) {}

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

  createSalesInvoice(invoice: SalesInvoice): Observable<SalesInvoice> {
    return this.http.post<SalesInvoice>(`${this.baseUrl}/sales-invoices`, invoice);
  }

  updateSalesInvoice(id: number, invoice: SalesInvoice): Observable<SalesInvoice> {
    return this.http.put<SalesInvoice>(`${this.baseUrl}/sales-invoices/${id}`, invoice);
  }
}
