import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService, CompanyProfile } from '../api.service';

@Component({
  selector: 'app-settings',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './settings.component.html',
  styleUrl: './settings.component.css'
})
export class SettingsComponent implements OnInit {
  profile: CompanyProfile = {
    company_name: '',
    address: '',
    gstin: '',
    pan: '',
    email: '',
    phone: ''
  };
  isLoading = true;
  isSaving = false;
  saveSuccess = false;

  constructor(private api: ApiService) {}

  ngOnInit() {
    this.api.getCompanyProfile().subscribe({
      next: (p) => {
        this.profile = p;
        this.isLoading = false;
      },
      error: (err) => {
        console.error(err);
        this.isLoading = false;
      }
    });
  }

  save() {
    this.isSaving = true;
    this.saveSuccess = false;
    this.api.saveCompanyProfile(this.profile).subscribe({
      next: (p) => {
        this.profile = p;
        this.isSaving = false;
        this.saveSuccess = true;
        setTimeout(() => this.saveSuccess = false, 3000);
      },
      error: (err) => {
        alert('Failed to save: ' + err.message);
        this.isSaving = false;
      }
    });
  }
}
