import { createHmac, timingSafeEqual } from "crypto";
import { revalidatePath, revalidateTag } from "next/cache";
import { NextRequest, NextResponse } from "next/server";

export const runtime = "nodejs";

export async function POST(req: NextRequest): Promise<NextResponse> {
  const secret = process.env.REVALIDATE_SECRET || "";
  if (!secret) {
    return NextResponse.json({ ok: false }, { status: 500 });
  }

  const rawBody = await req.text();
  const signature = req.headers.get("x-revalidate-signature") || "";
  const expected = createHmac("sha256", secret).update(rawBody).digest("hex");

  const sigBuf = Buffer.from(signature);
  const expBuf = Buffer.from(expected);
  if (sigBuf.length !== expBuf.length || !timingSafeEqual(sigBuf, expBuf)) {
    return NextResponse.json({ ok: false, error: "invalid_signature" }, { status: 401 });
  }

  try {
    const payload = JSON.parse(rawBody) as { tags?: string[] };
    for (const tag of payload.tags || []) {
      revalidateTag(tag);
    }
    revalidatePath("/", "layout");
    revalidatePath("/shop");
    revalidatePath("/contact");
    return NextResponse.json({ ok: true, revalidated: payload.tags || [] });
  } catch {
    return NextResponse.json({ ok: false }, { status: 400 });
  }
}
