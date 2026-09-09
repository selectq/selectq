import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService, SalesInvoice, BusinessPartner } from '../../api.service';

@Component({
  selector: 'app-sales-invoices-list',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './list.component.html',
  styleUrl: './list.component.css'
})
export class ListComponent implements OnInit {
  invoices: SalesInvoice[] = [];
  businessPartners: BusinessPartner[] = [];
  currencies: string[] = ['USD', 'GBP', 'EUR', 'AED', 'INR', 'CHF'];
  selectedInvoice: SalesInvoice | null = null;
  isNew = false;
  isLoading = true;

  constructor(private api: ApiService) {}

  ngOnInit() {
    this.loadData();
  }

  loadData() {
    this.isLoading = true;
    this.api.getBusinessPartners().subscribe({
      next: (bps) => {
        this.businessPartners = bps || [];
        this.api.getSalesInvoices().subscribe({
          next: (invs) => {
            this.invoices = invs || [];
            this.isLoading = false;
          },
          error: (err) => console.error(err)
        });
      },
      error: (err) => console.error(err)
    });
  }

  selectInvoice(inv: SalesInvoice) {
    this.selectedInvoice = { ...inv };
    this.isNew = false;
  }

  createNew() {
    this.selectedInvoice = {
      invoice_number: '',
      financial_year: '2026-27',
      business_partner_id: 0,
      invoice_date: new Date().toISOString().split('T')[0],
      currency: 'INR',
      amount: 0
    };
    this.isNew = true;
  }

  save() {
    if (!this.selectedInvoice) return;
    
    // Mandatory field validation
    if (!this.selectedInvoice.invoice_number || 
        !this.selectedInvoice.financial_year || 
        !this.selectedInvoice.business_partner_id || 
        !this.selectedInvoice.invoice_date || 
        !this.selectedInvoice.currency || 
        this.selectedInvoice.amount === null || 
        this.selectedInvoice.amount === undefined) {
      alert('All fields are mandatory. Please fill out all details.');
      return;
    }

    if (this.isNew) {
      this.api.createSalesInvoice(this.selectedInvoice).subscribe({
        next: () => {
          this.loadData();
          this.selectedInvoice = null;
        },
        error: (err) => alert('Failed to create: ' + err.message)
      });
    } else {
      this.api.updateSalesInvoice(this.selectedInvoice.id!, this.selectedInvoice).subscribe({
        next: () => {
          this.loadData();
          this.selectedInvoice = null;
        },
        error: (err) => alert('Failed to update: ' + err.message)
      });
    }
  }

  cancel() {
    this.selectedInvoice = null;
  }
}
