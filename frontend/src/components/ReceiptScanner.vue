<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import jsQR from 'jsqr'
import Modal from './Modal.vue'
import { receiptsApi } from '../api/receipts'
import { errMessage } from '../api/client'
import type { Receipt } from '../api/types'

const emit = defineEmits<{ close: []; saved: [] }>()

// Minimal shape of the (experimental) BarcodeDetector API.
interface DetectedBarcode {
  rawValue: string
}
interface BarcodeDetectorLike {
  detect(source: CanvasImageSource): Promise<DetectedBarcode[]>
}
type BarcodeDetectorCtor = new (opts?: { formats?: string[] }) => BarcodeDetectorLike

// Prefer the native BarcodeDetector (Chrome/Android); fall back to jsQR on a
// canvas everywhere else (e.g. iOS Safari), so auto-scan always works.
const barcodeCtor = (window as unknown as { BarcodeDetector?: BarcodeDetectorCtor }).BarcodeDetector

const video = ref<HTMLVideoElement | null>(null)
const qrUrl = ref('')
const detected = ref(false)
const cameraError = ref('')
const error = ref('')
const busy = ref(false)
// idle → uploading → processing → success | failed
const status = ref<'idle' | 'uploading' | 'processing' | 'success' | 'failed'>('idle')
const result = ref<Receipt | null>(null)

let stream: MediaStream | null = null
let detector: BarcodeDetectorLike | null = null
let scanCanvas: HTMLCanvasElement | null = null // reused by the jsQR fallback
let scanning = false
let scanTimer: ReturnType<typeof setTimeout> | undefined
let pollTimer: ReturnType<typeof setTimeout> | undefined

async function startCamera() {
  cameraError.value = ''
  qrUrl.value = ''
  detected.value = false
  try {
    stream = await navigator.mediaDevices.getUserMedia({
      video: { facingMode: { ideal: 'environment' } },
      audio: false,
    })
    if (video.value) {
      video.value.srcObject = stream
      await video.value.play()
    }
    if (barcodeCtor) detector = new barcodeCtor({ formats: ['qr_code'] })
    scanning = true
    scanLoop()
  } catch (e) {
    cameraError.value = errMessage(e)
  }
}

// detectFrame returns the QR value in the current video frame, or null. It uses
// the native detector when available, otherwise jsQR over a downscaled canvas.
async function detectFrame(): Promise<string | null> {
  const v = video.value
  if (!v || !v.videoWidth) return null

  if (detector) {
    try {
      const codes = await detector.detect(v)
      return codes[0]?.rawValue?.trim() || null
    } catch {
      return null
    }
  }

  // jsQR fallback: draw the frame (capped for speed) and scan its pixels.
  if (!scanCanvas) scanCanvas = document.createElement('canvas')
  const scale = Math.min(1, 720 / v.videoWidth)
  const w = Math.round(v.videoWidth * scale)
  const h = Math.round(v.videoHeight * scale)
  scanCanvas.width = w
  scanCanvas.height = h
  const ctx = scanCanvas.getContext('2d', { willReadFrequently: true })
  if (!ctx) return null
  ctx.drawImage(v, 0, 0, w, h)
  const image = ctx.getImageData(0, 0, w, h)
  return jsQR(image.data, w, h, { inversionAttempts: 'dontInvert' })?.data?.trim() || null
}

function scanLoop() {
  if (!scanning) return
  detectFrame()
    .then((value) => {
      if (value) {
        qrUrl.value = value
        detected.value = true
        scanning = false
      }
    })
    .finally(() => {
      if (scanning) scanTimer = setTimeout(scanLoop, 300)
    })
}

function stopCamera() {
  scanning = false
  if (scanTimer) clearTimeout(scanTimer)
  stream?.getTracks().forEach((t) => t.stop())
  stream = null
}

function grabPhoto(): Promise<Blob> {
  return new Promise((resolve, reject) => {
    const v = video.value
    if (!v || !v.videoWidth) {
      reject(new Error('Camera not ready.'))
      return
    }
    const canvas = document.createElement('canvas')
    canvas.width = v.videoWidth
    canvas.height = v.videoHeight
    const ctx = canvas.getContext('2d')
    if (!ctx) {
      reject(new Error('Could not capture the frame.'))
      return
    }
    ctx.drawImage(v, 0, 0)
    canvas.toBlob((b) => (b ? resolve(b) : reject(new Error('Could not capture the frame.'))), 'image/jpeg', 0.9)
  })
}

async function capture() {
  error.value = ''
  if (!qrUrl.value.trim()) {
    error.value = 'Point the camera at the receipt QR code, or paste its URL below.'
    return
  }
  busy.value = true
  status.value = 'uploading'
  try {
    const photo = await grabPhoto()
    const { id } = await receiptsApi.create(qrUrl.value.trim(), photo)
    stopCamera()
    status.value = 'processing'
    await poll(id)
  } catch (e) {
    error.value = errMessage(e)
    status.value = 'idle'
  } finally {
    busy.value = false
  }
}

async function poll(id: string) {
  const deadline = Date.now() + 40_000
  while (Date.now() < deadline) {
    const r = await receiptsApi.get(id)
    if (r.status === 'success') {
      result.value = r
      status.value = 'success'
      emit('saved')
      return
    }
    if (r.status === 'failed') {
      result.value = r
      status.value = 'failed'
      error.value = r.error || 'The receipt could not be parsed.'
      return
    }
    await new Promise<void>((res) => {
      pollTimer = setTimeout(res, 1500)
    })
  }
  // Still processing after the timeout — leave it; the user can check later.
}

function close() {
  stopCamera()
  if (pollTimer) clearTimeout(pollTimer)
  emit('close')
}

onMounted(startCamera)
onUnmounted(() => {
  stopCamera()
  if (pollTimer) clearTimeout(pollTimer)
})
</script>

<template>
  <Modal title="Scan receipt" size="lg" @close="close">
    <!-- result states -->
    <div v-if="status === 'success' && result" class="space-y-4 text-center">
      <div class="mx-auto grid h-14 w-14 place-items-center rounded-full bg-emerald-100 text-emerald-600">
        <i class="ti ti-check text-3xl" />
      </div>
      <div>
        <p class="text-lg font-semibold text-slate-900">{{ result.merchant_name || 'Receipt saved' }}</p>
        <p v-if="result.total_amount" class="text-2xl font-bold text-slate-900">{{ result.total_amount.format() }}</p>
        <p class="mt-1 text-sm text-slate-500">{{ result.items.length }} item{{ result.items.length === 1 ? '' : 's' }}</p>
      </div>
      <button class="btn btn-primary w-full" @click="close">Done</button>
    </div>

    <div v-else-if="status === 'failed'" class="space-y-4 text-center">
      <div class="mx-auto grid h-14 w-14 place-items-center rounded-full bg-rose-100 text-rose-600">
        <i class="ti ti-alert-triangle text-3xl" />
      </div>
      <p class="text-sm text-slate-600">{{ error }}</p>
      <button class="btn btn-primary w-full" @click="close">Close</button>
    </div>

    <!-- capture / scanning -->
    <div v-else class="space-y-4">
      <div class="relative aspect-[3/4] w-full overflow-hidden rounded-2xl bg-slate-900">
        <video ref="video" class="h-full w-full object-cover" playsinline muted />

        <!-- scan reticle -->
        <div class="pointer-events-none absolute inset-0 grid place-items-center">
          <div
            class="h-48 w-48 rounded-2xl border-2 transition"
            :class="detected ? 'border-emerald-400' : 'border-white/70'"
          />
        </div>

        <!-- detection / processing badges -->
        <div
          v-if="detected"
          class="absolute top-3 left-1/2 -translate-x-1/2 rounded-full bg-emerald-500 px-3 py-1 text-xs font-semibold text-white shadow"
        >
          <i class="ti ti-qrcode" /> QR detected
        </div>
        <div
          v-if="status === 'processing'"
          class="absolute inset-0 grid place-items-center bg-slate-900/60 text-center text-sm font-medium text-white"
        >
          <div class="space-y-2">
            <i class="ti ti-loader-2 animate-spin text-3xl" />
            <p>Reading receipt…</p>
          </div>
        </div>
      </div>

      <p v-if="cameraError" class="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-600">{{ cameraError }}</p>
      <p v-else class="text-center text-xs text-slate-500">Point the camera at the receipt’s QR code.</p>

      <!-- manual / detected URL -->
      <input
        v-model="qrUrl"
        type="url"
        inputmode="url"
        autocomplete="off"
        autocorrect="off"
        spellcheck="false"
        placeholder="https://ofd.soliq.uz/check?..."
        class="field text-sm"
      />

      <p v-if="error" class="text-sm text-rose-600">{{ error }}</p>

      <button class="btn btn-primary w-full" :disabled="busy || status === 'processing'" @click="capture">
        <i v-if="busy || status === 'processing'" class="ti ti-loader-2 animate-spin" />
        <i v-else class="ti ti-camera" />
        {{ status === 'uploading' ? 'Uploading…' : status === 'processing' ? 'Processing…' : 'Capture & save' }}
      </button>
    </div>
  </Modal>
</template>
