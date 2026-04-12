import { useRef, useEffect, useState } from 'react'
import * as THREE from 'three'
import { useAppStore } from '../store/useAppStore'

const STEPS = [
  { title: 'Welcome to OMEGA PRIME', desc: 'Elite institutional trading. Zero compromise.' },
  { title: 'Authentication', desc: 'Sign in securely to access your trading environment.' },
  { title: 'Risk Profile', desc: 'Configure your risk tolerance. Hard limits protect your capital.' },
  { title: 'Exchange API Keys', desc: 'Connect Kraken, Binance, or Coinbase with read+trade scope only.' },
  { title: 'Wallet Connect', desc: 'Optional: connect MetaMask or Phantom for DeFi features.' },
  { title: 'Paper Trading', desc: 'Start safely with simulated trades before going live.' },
  { title: 'System Check', desc: 'Testing connectivity to all systems...' },
  { title: 'Ready', desc: 'OMEGA PRIME is initialized. Your risk limits are active.' },
]

function OmegaCanvas() {
  const ref = useRef<HTMLCanvasElement>(null)
  useEffect(() => {
    const canvas = ref.current
    if (!canvas) return
    const renderer = new THREE.WebGLRenderer({ canvas, alpha: true, antialias: true })
    renderer.setSize(300, 300)
    const scene  = new THREE.Scene()
    const camera = new THREE.PerspectiveCamera(60, 1, 0.1, 100)
    camera.position.z = 3

    const geo  = new THREE.TorusGeometry(1, 0.15, 32, 100)
    const mat  = new THREE.MeshStandardMaterial({ color: 0x6dbbff, roughness: 0.1, metalness: 0.9 })
    const torus = new THREE.Mesh(geo, mat)
    scene.add(torus)

    const light1 = new THREE.PointLight(0x6dbbff, 3, 10)
    light1.position.set(2, 2, 2)
    scene.add(light1)
    const light2 = new THREE.PointLight(0x4df2b2, 2, 10)
    light2.position.set(-2, -1, 1)
    scene.add(light2)
    scene.add(new THREE.AmbientLight(0x1a2740, 1))

    let frame = 0
    const animate = () => {
      frame = requestAnimationFrame(animate)
      torus.rotation.x += 0.005
      torus.rotation.y += 0.012
      renderer.render(scene, camera)
    }
    animate()
    return () => { cancelAnimationFrame(frame); renderer.dispose() }
  }, [])
  return <canvas ref={ref} width={300} height={300} />
}

export function Onboarding() {
  const [step, setStep] = useState(0)
  const [maxRisk, setMaxRisk] = useState(2)
  const [maxDD, setMaxDD] = useState(10)
  const [paper, setPaper] = useState(true)
  const setOnboardingComplete = useAppStore((s) => s.setOnboardingComplete)

  const next = () => {
    if (step < STEPS.length - 1) setStep((s) => s + 1)
    else setOnboardingComplete(true)
  }

  return (
    <div className="min-h-screen flex flex-col items-center justify-center bg-bg text-text p-6">
      {/* Progress */}
      <div className="flex gap-1.5 mb-8">
        {STEPS.map((_, i) => (
          <div key={i} className={`h-1 rounded-full transition-all ${i <= step ? 'bg-blue w-6' : 'bg-border w-3'}`} />
        ))}
      </div>

      <div className="w-full max-w-md flex flex-col items-center gap-6">
        {/* Three.js on first step */}
        {step === 0 && <OmegaCanvas />}

        {/* Step content */}
        <div className="text-center">
          <h1 className="text-2xl font-bold mb-2">{STEPS[step].title}</h1>
          <p className="text-muted">{STEPS[step].desc}</p>
        </div>

        {/* Step-specific content */}
        {step === 2 && (
          <div className="w-full space-y-4">
            <div>
              <label className="kicker">Max Daily Loss: {maxRisk}%</label>
              <input type="range" min={1} max={5} value={maxRisk} onChange={(e) => setMaxRisk(+e.target.value)}
                className="w-full" />
            </div>
            <div>
              <label className="kicker">Max Drawdown: {maxDD}%</label>
              <input type="range" min={5} max={20} value={maxDD} onChange={(e) => setMaxDD(+e.target.value)}
                className="w-full" />
            </div>
            <div className="flex gap-2">
              {['Conservative', 'Balanced', 'Extreme'].map((preset) => (
                <button key={preset} onClick={() => {
                  if (preset === 'Conservative') { setMaxRisk(1); setMaxDD(5) }
                  else if (preset === 'Balanced') { setMaxRisk(2); setMaxDD(10) }
                  else { setMaxRisk(5); setMaxDD(20) }
                }} className="flex-1 text-xs border border-border rounded-lg py-1.5 hover:border-blue/50 hover:text-blue transition-all">
                  {preset}
                </button>
              ))}
            </div>
          </div>
        )}

        {step === 5 && (
          <div className="w-full border border-border rounded-xl p-4 flex items-center justify-between">
            <div>
              <p className="font-semibold">Paper Trading Mode</p>
              <p className="text-xs text-muted">Recommended for first-time setup</p>
            </div>
            <button onClick={() => setPaper(!paper)}
              className={`w-12 h-6 rounded-full transition-all ${paper ? 'bg-green/30 border border-green/50' : 'bg-border/40 border border-border'}`}>
              <span className={`block w-5 h-5 rounded-full transition-all m-0.5 ${paper ? 'translate-x-6 bg-green' : 'bg-muted'}`} />
            </button>
          </div>
        )}

        {step === 6 && (
          <div className="w-full space-y-2">
            {['WebSocket', 'Kafka', 'Risk Engine', 'Market Data', 'AI Models'].map((sys) => (
              <div key={sys} className="flex items-center justify-between border border-border/50 rounded-lg p-2">
                <span className="text-sm">{sys}</span>
                <span className="text-xs text-green flex items-center gap-1.5">
                  <span className="w-1.5 h-1.5 rounded-full bg-green inline-block animate-pulse" />
                  Connected
                </span>
              </div>
            ))}
          </div>
        )}

        {/* CTA */}
        <button onClick={next}
          className="w-full py-3 rounded-xl font-bold text-base bg-blue/20 border border-blue/60 text-blue hover:bg-blue/30 transition-all">
          {step === STEPS.length - 1 ? 'Launch Dashboard' : 'Continue'}
        </button>

        {step === 0 && (
          <p className="text-xs text-muted text-center">Elite Engineering Collective · April 2026</p>
        )}
      </div>
    </div>
  )
}
