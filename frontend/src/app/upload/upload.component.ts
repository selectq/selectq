import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ApiService } from '../api.service';
import { Router } from '@angular/router';

@Component({
  selector: 'app-upload',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './upload.component.html',
  styleUrls: ['./upload.component.css']
})
export class UploadComponent {
  selectedFile: File | null = null;
  isUploading = false;
  uploadSuccess: any = null;
  errorMessage = '';

  constructor(private api: ApiService, private router: Router) {}

  onFileSelected(event: any) {
    const file = event.target.files[0];
    if (file) {
      this.selectedFile = file;
      this.uploadSuccess = null;
      this.errorMessage = '';
    }
  }

  onUpload() {
    if (!this.selectedFile) return;

    this.isUploading = true;
    this.errorMessage = '';
    
    this.api.uploadStatement(this.selectedFile).subscribe({
      next: (res) => {
        this.isUploading = false;
        this.uploadSuccess = res;
      },
      error: (err) => {
        this.isUploading = false;
        this.errorMessage = err.error || 'Failed to upload statement';
      }
    });
  }

  goToHistory() {
    this.router.navigate(['/history']);
  }
}
