"use client";

import React, { useState, Suspense } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { ArrowRight, AlertCircle } from "lucide-react";
import { apiPost } from "@/lib/api/client";
import { useAuthStore } from "@/store/useAuthStore";
import { useCartStore } from "@/store/useCartStore";
import type { UserDTO } from "@/lib/api/mappers";

function RegisterForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const redirectUrl = searchParams.get("redirect") || "/account";

  const fetchCustomerMe = useAuthStore((s) => s.fetchCustomerMe);
  const mergeWithServer = useCartStore((s) => s.mergeWithServer);

  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMsg(null);

    if (password !== confirmPassword) {
      setErrorMsg("Konfirmasi password tidak cocok.");
      return;
    }

    if (password.length < 8) {
      setErrorMsg("Password minimal terdiri dari 8 karakter.");
      return;
    }

    setLoading(true);

    try {
      await apiPost<{ user: UserDTO }>("/v1/auth/register", {
        name: name.trim(),
        email: email.trim(),
        phone: phone.trim() || undefined,
        password,
      });

      await fetchCustomerMe();
      await mergeWithServer();
      router.push(redirectUrl);
    } catch (err: unknown) {
      setErrorMsg(
        err instanceof Error
          ? err.message
          : "Pendaftaran gagal. Pastikan email belum terdaftar sebelumnya."
      );
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="py-16 md:py-24 bg-bg min-h-[75vh] flex items-center justify-center">
      <div className="w-full max-w-md px-6">
        <div className="text-center mb-8">
          <p className="micro-label text-ink-3">PENDAFTARAN</p>
          <h1 className="heading-sub text-ink mt-2">Buat Akun Pelanggan.</h1>
          <p className="text-sm text-ink-2 mt-2">
            Nikmati kemudahan checkout, riwayat nota pesanan, dan penawaran eksklusif.
          </p>
        </div>

        {errorMsg && (
          <div className="mb-6 p-4 bg-danger/10 border border-danger/30 text-danger text-xs flex items-center gap-2.5">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{errorMsg}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="micro-label text-ink-3 block mb-1">
              NAMA LENGKAP *
            </label>
            <input
              type="text"
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Contoh: Andi Pratama"
              className="input-boxed text-sm"
            />
          </div>

          <div>
            <label className="micro-label text-ink-3 block mb-1">
              ALAMAT EMAIL *
            </label>
            <input
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="andi@email.com"
              className="input-boxed text-sm"
            />
          </div>

          <div>
            <label className="micro-label text-ink-3 block mb-1">
              NOMOR WHATSAPP *
            </label>
            <input
              type="tel"
              required
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              placeholder="081234567890"
              className="input-boxed text-sm"
            />
          </div>

          <div>
            <label className="micro-label text-ink-3 block mb-1">
              PASSWORD (MIN. 8 KARAKTER) *
            </label>
            <input
              type="password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              className="input-boxed text-sm"
            />
          </div>

          <div>
            <label className="micro-label text-ink-3 block mb-1">
              KONFIRMASI PASSWORD *
            </label>
            <input
              type="password"
              required
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              placeholder="••••••••"
              className="input-boxed text-sm"
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="btn-pill-primary w-full text-center mt-2 inline-flex items-center justify-center gap-2"
          >
            <span>{loading ? "Mendaftarkan..." : "Daftar Akun"}</span>
            <ArrowRight className="w-4 h-4" />
          </button>
        </form>

        <div className="mt-8 pt-6 border-t border-line text-center text-sm text-ink-2">
          <span>Sudah memiliki akun? </span>
          <Link
            href={`/login${redirectUrl ? `?redirect=${encodeURIComponent(redirectUrl)}` : ""}`}
            className="font-medium text-ink underline hover:opacity-75"
          >
            Masuk di sini
          </Link>
        </div>
      </div>
    </div>
  );
}

export default function RegisterPage() {
  return (
    <Suspense
      fallback={
        <div className="py-24 text-center min-h-[60vh] flex items-center justify-center">
          <p className="text-sm text-ink-3">Memuat pendaftaran...</p>
        </div>
      }
    >
      <RegisterForm />
    </Suspense>
  );
}
