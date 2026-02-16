import { Component, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';

interface SearchResult {
  file_path: string;
  mod_time: string;
}

interface SearchResponse {
  query: string;
  dir: string;
  files: SearchResult[];
  error?: string;
}

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <div style="font-family: sans-serif; padding: 20px; max-width: 800px; margin: 0 auto;">
      <h1>NFO Searcher</h1>

      <div style="display: flex; gap: 10px; margin-bottom: 20px;">
        <input
          [ngModel]="query()"
          (ngModelChange)="query.set($event)"
          placeholder="Search string"
          style="flex: 1; padding: 8px;"
          (keyup.enter)="search()"
        >
        <input
          [ngModel]="dir()"
          (ngModelChange)="dir.set($event)"
          placeholder="Directory (e.g. .)"
          style="width: 200px; padding: 8px;"
          (keyup.enter)="search()"
        >
        <button (click)="search()" [disabled]="loading()" style="padding: 8px 16px; cursor: pointer;">
          {{ loading() ? 'Searching...' : 'Search' }}
        </button>
      </div>

      <div *ngIf="error()" style="color: red; margin-bottom: 20px;">
        {{ error() }}
      </div>

      <div *ngIf="results().length > 0">
        <h3>Results (Sorted by Newest)</h3>
        <table style="width: 100%; border-collapse: collapse;">
          <thead>
            <tr style="text-align: left; border-bottom: 2px solid #ccc;">
              <th style="padding: 10px;">File Path</th>
              <th style="padding: 10px;">Modified</th>
            </tr>
          </thead>
          <tbody>
            <tr *ngFor="let file of results()" style="border-bottom: 1px solid #eee;">
              <td style="padding: 10px;">{{ file.file_path }}</td>
              <td style="padding: 10px;">{{ file.mod_time | date:'medium' }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div *ngIf="!loading() && results().length === 0 && searched()" style="margin-top: 20px; color: #666;">
        No .nfo files found containing "{{ query() }}" in {{ dir() }}.
      </div>
    </div>
  `,
  styles: [`
    input:focus { outline: 2px solid #007bff; }
    button:hover { background-color: #f0f0f0; }
    button:disabled { cursor: not-allowed; opacity: 0.6; }
  `],
})
export class App {
  query = signal('');
  dir = signal('.');
  loading = signal(false);
  searched = signal(false);
  results = signal<SearchResult[]>([]);
  error = signal<string | null>(null);

  async search() {
    if (!this.query() || !this.dir()) return;

    this.loading.set(true);
    this.error.set(null);
    this.searched.set(false);

    try {
      const resp = await fetch(`/search/?q=${encodeURIComponent(this.query())}&dir=${encodeURIComponent(this.dir())}`);
      if (!resp.ok) {
        const data = await resp.json();
        throw new Error(data.error || `Server returned ${resp.status}`);
      }
      const data: SearchResponse = await resp.json();
      this.results.set(data.files || []);
      this.searched.set(true);
    } catch (e: any) {
      this.error.set(e.message);
    } finally {
      this.loading.set(false);
    }
  }
}
