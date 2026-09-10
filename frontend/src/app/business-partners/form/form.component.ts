import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule, FormBuilder, FormGroup, Validators, FormArray } from '@angular/forms';
import { ApiService, BusinessPartner } from '../../api.service';
import { Router, ActivatedRoute } from '@angular/router';
import { debounceTime, distinctUntilChanged, Subject, takeUntil } from 'rxjs';

@Component({
  selector: 'app-form',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule],
  templateUrl: './form.component.html',
  styleUrls: ['./form.component.css']
})
export class FormComponent implements OnInit, OnDestroy {
  bpForm: FormGroup;
  isSubmitting = false;
  isEditMode = false;
  partnerId: number | null = null;
  similarPartners: BusinessPartner[] = [];
  private destroy$ = new Subject<void>();

  constructor(
    private fb: FormBuilder,
    private api: ApiService,
    private router: Router,
    private route: ActivatedRoute
  ) {
    this.bpForm = this.fb.group({
      name: ['', Validators.required],
      billing_address: [''],
      invoice_currency: ['USD', Validators.required],
      tax_information: [''],
      contacts: this.fb.array([this.createContactGroup(true)])
    });
  }

  get contacts(): FormArray {
    return this.bpForm.get('contacts') as FormArray;
  }

  createContactGroup(isPrimary: boolean = false): FormGroup {
    return this.fb.group({
      name: ['', Validators.required],
      email: [''],
      phone: [''],
      is_primary: [isPrimary]
    });
  }

  addContact() {
    this.contacts.push(this.createContactGroup());
  }

  removeContact(index: number) {
    this.contacts.removeAt(index);
    if (this.contacts.length === 0) {
      this.addContact(); // Ensure at least one
    }
    this.ensurePrimary();
  }

  setPrimary(index: number) {
    for (let i = 0; i < this.contacts.length; i++) {
      this.contacts.at(i).get('is_primary')?.setValue(i === index);
    }
  }

  private ensurePrimary() {
    // Check if any is primary
    const hasPrimary = this.contacts.controls.some(c => c.get('is_primary')?.value);
    if (!hasPrimary && this.contacts.length > 0) {
      this.contacts.at(0).get('is_primary')?.setValue(true);
    }
  }

  ngOnInit() {
    this.route.paramMap.subscribe(params => {
      const id = params.get('id');
      if (id) {
        this.isEditMode = true;
        this.partnerId = +id;
        this.api.getBusinessPartner(this.partnerId).subscribe({
          next: (p) => {
            // clear existing
            this.contacts.clear();
            if (p.contacts && p.contacts.length > 0) {
              p.contacts.forEach(c => {
                this.contacts.push(this.fb.group({
                  name: [c.name, Validators.required],
                  email: [c.email],
                  phone: [c.phone],
                  is_primary: [c.is_primary]
                }));
              });
            } else {
              this.contacts.push(this.createContactGroup(true));
            }

            this.bpForm.patchValue({
              name: p.name,
              billing_address: p.billing_address,
              invoice_currency: p.invoice_currency,
              tax_information: p.tax_information
            });
          },
          error: (err) => console.error('Failed to load partner', err)
        });
      }
    });

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

  onSubmit() {
    if (this.bpForm.invalid) return;

    this.isSubmitting = true;
    
    if (this.isEditMode && this.partnerId) {
      this.api.updateBusinessPartner(this.partnerId, this.bpForm.value).subscribe({
        next: () => this.router.navigate(['/business-partners']),
        error: (err) => {
          console.error('Failed to update partner', err);
          alert('Failed to update. Duplicate name?');
          this.isSubmitting = false;
        }
      });
    } else {
      this.api.createBusinessPartner(this.bpForm.value).subscribe({
        next: () => this.router.navigate(['/business-partners']),
        error: (err) => {
          console.error('Failed to create partner', err);
          alert('Failed to save. Duplicate name?');
          this.isSubmitting = false;
        }
      });
    }
  }

  cancel() {
    this.router.navigate(['/business-partners']);
  }
}
