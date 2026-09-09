import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule, FormBuilder, FormGroup, Validators } from '@angular/forms';
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
      tax_information: ['']
    });
  }

  ngOnInit() {
    this.route.paramMap.subscribe(params => {
      const id = params.get('id');
      if (id) {
        this.isEditMode = true;
        this.partnerId = +id;
        this.api.getBusinessPartner(this.partnerId).subscribe({
          next: (p) => {
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
