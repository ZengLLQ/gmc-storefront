import type { UseFormSetError, FieldValues, Path } from "react-hook-form";

export interface ErrorEnvelopePayload {
  error?: {
    code?: string;
    message?: string;
    fields?: Record<string, string>;
    requestId?: string;
  };
}

const DEFAULT_MESSAGES: Record<string, string> = {
  validation_failed: "Mohon periksa kembali data yang Anda masukkan.",
  bad_request: "Permintaan tidak valid.",
  unauthorized: "Sesi Anda telah berakhir. Silakan masuk kembali.",
  invalid_credentials: "Email atau password salah.",
  email_taken: "Email ini sudah terdaftar.",
  forbidden: "Anda tidak memiliki izin untuk melakukan tindakan ini.",
  not_found: "Data yang Anda cari tidak ditemukan.",
  out_of_stock:
    "Maaf, stok untuk salah satu produk baru saja berubah. Silakan periksa kembali keranjang Anda.",
  price_changed:
    "Harga produk di keranjang telah diperbarui. Silakan tinjau kembali total pesanan Anda.",
  voucher_invalid: "Kode voucher tidak valid atau tidak memenuhi syarat.",
  conflict:
    "Data telah diperbarui oleh pengguna lain. Silakan muat ulang halaman.",
  rate_limited:
    "Terlalu banyak percobaan dalam waktu singkat. Silakan tunggu sebentar.",
  internal: "Terjadi kendala pada server. Silakan coba beberapa saat lagi.",
};

export class ApiError extends Error {
  public readonly status: number;
  public readonly code: string;
  public readonly fields?: Record<string, string>;
  public readonly requestId?: string;

  constructor(params: {
    status: number;
    code?: string;
    message?: string;
    fields?: Record<string, string>;
    requestId?: string;
  }) {
    const code = params.code || "internal";
    const msg =
      params.message ||
      DEFAULT_MESSAGES[code] ||
      "Terjadi kesalahan. Silakan coba lagi.";
    super(msg);
    this.name = "ApiError";
    this.status = params.status;
    this.code = code;
    this.fields = params.fields;
    this.requestId = params.requestId;
  }
}

export function applyFieldErrors<T extends FieldValues>(
  err: unknown,
  setError: UseFormSetError<T>
): string {
  if (err instanceof ApiError) {
    if (err.fields) {
      for (const [field, msg] of Object.entries(err.fields)) {
        setError(field as Path<T>, { type: "server", message: msg });
      }
    }
    return err.message;
  }
  if (err instanceof Error) {
    return err.message;
  }
  return "Terjadi kesalahan yang tidak terduga.";
}
