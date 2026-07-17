import { Component, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';

import { AuthService } from '../../core/auth.service';

// Página acessada pelo link enviado em AuthUsecase.ForgotPassword
// (?token=... na URL) — troca a senha e não faz login automático, para o usuário
// entrar deliberadamente com a senha nova em seguida.
@Component({
  selector: 'app-reset-password',
  standalone: true,
  imports: [CommonModule, FormsModule, RouterLink, MatButtonModule, MatFormFieldModule, MatInputModule, MatProgressSpinnerModule],
  templateUrl: './reset-password.component.html',
})
export class ResetPasswordComponent implements OnInit {
  token = '';
  newPassword = '';
  confirmPassword = '';

  loading = signal(false);
  success = signal(false);
  error = signal<string | null>(null);
  missingToken = signal(false);

  constructor(
    private route: ActivatedRoute,
    private router: Router,
    private auth: AuthService,
  ) {}

  ngOnInit(): void {
    const token = this.route.snapshot.queryParamMap.get('token');
    if (!token) {
      this.missingToken.set(true);
      return;
    }
    this.token = token;
  }

  submit(): void {
    this.error.set(null);
    if (this.newPassword.length < 6) {
      this.error.set('A senha precisa ter pelo menos 6 caracteres.');
      return;
    }
    if (this.newPassword !== this.confirmPassword) {
      this.error.set('As senhas não coincidem.');
      return;
    }

    this.loading.set(true);
    this.auth.resetPassword(this.token, this.newPassword).subscribe({
      next: () => {
        this.loading.set(false);
        this.success.set(true);
      },
      error: err => {
        this.loading.set(false);
        this.error.set(err?.error?.error ?? 'Não foi possível redefinir a senha. O link pode ter expirado.');
      },
    });
  }

  irParaLogin(): void {
    this.router.navigateByUrl('/assinatura');
  }
}
