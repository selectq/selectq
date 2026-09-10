import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule, FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ApiService, BusinessPartner } from '../../api.service';
import { debounceTime, distinctUntilChanged, Subject, takeUntil } from 'rxjs';

@Component({
  selector: 'app-list',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule],
  templateUrl: './list.component.html',
  styleUrls: ['./list.component.css']
})
export class ListComponent implements OnInit, OnDestroy {
  partners: BusinessPartner[] = [];
  selectedPartner: BusinessPartner | null = null;
  isLoading = true;

  // Inline form state
  isEditing = false;
  isEditMode = false;
  isSubmitting = false;
  editingPartnerId: number | null = null;
  similarPartners: BusinessPartner[] = [];
  bpForm: FormGroup;
  private destroy$ = new Subject<void>();

  get formAvatarLetter(): string {
    const name = this.bpForm.get('name')?.value;
    return name ? name.charAt(0).toUpperCase() : '+';
  }

  constructor(private api: ApiService, private fb: FormBuilder) {
    this.bpForm = this.fb.group({
      name: ['', Validators.required],
      billing_address: [''],
      invoice_currency: ['USD', Validators.required],
      tax_information: ['']
    });
  }

  ngOnInit() {
    this.loadPartners();

    // Watch name field for duplicate detection
    this.bpForm.get('name')?.valueChanges.pipe(
      debounceTime(300),
      distinctUntilChanged(),
      takeUntil(this.destroy$)
    ).subscribe(name => {
      if (!this.isEditMode && name && name.length > 1) {
        this.api.getBusinessPartners(name).subscribe(data => {
          this.similarPartners = data || [];
        });
      } else {
        this.similarPartners = [];
      }
    });
  }

  ngOnDestroy() {
    this.destroy$.next();
    this.destroy$.complete();
  }

  loadPartners() {
    this.isLoading = true;
    this.api.getBusinessPartners().subscribe({
      next: (data) => {
        this.partners = data || [];
        this.isLoading = false;
        // Re-select current partner if it still exists (after save)
        if (this.selectedPartner) {
          const found = this.partners.find(p => p.id === this.selectedPartner!.id);
          this.selectedPartner = found || (this.partners.length > 0 ? this.partners[0] : null);
        } else if (this.partners.length > 0) {
          this.selectedPartner = this.partners[0];
        }
      },
      error: (err) => {
        console.error('Error fetching partners', err);
        this.isLoading = false;
      }
    });
  }

  selectPartner(p: BusinessPartner) {
    if (this.isEditing) {
      // If editing, switch to view mode first
      this.cancelEdit();
    }
    this.selectedPartner = p;
  }

  addNew() {
    this.isEditing = true;
    this.isEditMode = false;
    this.editingPartnerId = null;
    this.similarPartners = [];
    this.bpForm.reset({ name: '', billing_address: '', invoice_currency: 'USD', tax_information: '' });
  }

  editPartner() {
    if (!this.selectedPartner) return;
    this.isEditing = true;
    this.isEditMode = true;
    this.editingPartnerId = this.selectedPartner.id!;
    this.similarPartners = [];
    this.bpForm.patchValue({
      name: this.selectedPartner.name,
      billing_address: this.selectedPartner.billing_address,
      invoice_currency: this.selectedPartner.invoice_currency,
      tax_information: this.selectedPartner.tax_information
    });
  }

  cancelEdit() {
    this.isEditing = false;
    this.isEditMode = false;
    this.editingPartnerId = null;
    this.similarPartners = [];
    this.isSubmitting = false;
  }

  onSubmit() {
    if (this.bpForm.invalid) return;
    this.isSubmitting = true;

    if (this.isEditMode && this.editingPartnerId) {
      this.api.updateBusinessPartner(this.editingPartnerId, this.bpForm.value).subscribe({
        next: () => {
          this.isEditing = false;
          this.isSubmitting = false;
          this.loadPartners();
        },
        error: (err) => {
          console.error('Failed to update partner', err);
          alert('Failed to update. Duplicate name?');
          this.isSubmitting = false;
        }
      });
    } else {
      this.api.createBusinessPartner(this.bpForm.value).subscribe({
        next: (created) => {
          this.isEditing = false;
          this.isSubmitting = false;
          // Select newly created partner after reload
          this.selectedPartner = created;
          this.loadPartners();
        },
        error: (err) => {
          console.error('Failed to create partner', err);
          alert('Failed to save. Duplicate name?');
          this.isSubmitting = false;
        }
      });
    }
  }

  confirmDelete() {
    // Simple confirm for now — could be replaced with a modal
    if (!this.selectedPartner?.id) return;
    if (!confirm(`Delete "${this.selectedPartner.name}"? This cannot be undone.`)) return;

    this.api.deleteBusinessPartner(this.selectedPartner.id).subscribe({
      next: () => {
        this.selectedPartner = null;
        this.loadPartners();
      },
      error: (err) => {
        console.error('Failed to delete partner', err);
        alert('Failed to delete partner.');
      }
    });
  }
}
