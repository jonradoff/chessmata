import { useRef, useEffect, useMemo } from 'react'
import { useThree, useFrame } from '@react-three/fiber'
import { OrbitControls, Environment } from '@react-three/drei'
import * as THREE from 'three'
import gsap from 'gsap'
import { ChessBoard } from './ChessBoard'
import { ChessPieces } from './ChessPieces'
import type { useGameState } from '../hooks/useGameState'
import type { Settings } from '../hooks/useSettings'
import type { MakeMoveResponse } from '../api/gameApi'
import type { PieceType } from '../utils/chessRules'

export interface PendingPromotion {
  from: string
  to: string
  pieceId: string
  isWhite: boolean
  toFile: number
  toRank: number
}

export interface OnlineContext {
  sessionId: string | null
  playerId: string | null
  playerColor: 'white' | 'black' | null
  currentTurn: 'white' | 'black' | null
  gameStatus: 'waiting' | 'active' | 'complete' | null
  makeMove?: (from: string, to: string, promotion?: string) => Promise<MakeMoveResponse>
  onInvalidMove?: (info: { reason: string; pieceType: PieceType }) => void
  onPendingPromotion?: (info: PendingPromotion) => void
}

interface ChessSceneProps {
  is3D: boolean
  isTransitioning: boolean
  onTransitionComplete: () => void
  gameState: ReturnType<typeof useGameState>
  settings: Settings
  onlineContext?: OnlineContext
}

export function ChessScene({ is3D, isTransitioning, onTransitionComplete, gameState, settings, onlineContext }: ChessSceneProps) {
  const { camera } = useThree()
  const controlsRef = useRef<any>(null)
  const groupRef = useRef<THREE.Group>(null)
  const morphProgressRef = useRef({ value: is3D ? 1 : 0 })
  const prevIs3DRef = useRef(is3D)

  // Rotate board when playing as white so player's pieces are at the bottom
  // Default view has black at bottom, so rotate 180° for white players
  const boardRotation = onlineContext?.playerColor === 'white' ? Math.PI : 0

  // Camera positions
  const camera3D = useMemo(() => ({ position: new THREE.Vector3(0, 8, 12), target: new THREE.Vector3(0, 0, 0) }), [])
  // Moderate height + wider FOV: negligible perspective distortion (<0.5%), smooth animation
  // halfHeight = 50 * tan(11.75°) ≈ 10.4 → board at ~38% of viewport (60% of previous)
  const camera2D = useMemo(() => ({ position: new THREE.Vector3(0, 50, 0), target: new THREE.Vector3(0, 0, 0), fov: 23.5 }), [])

  useEffect(() => {
    if (prevIs3DRef.current === is3D) return
    prevIs3DRef.current = is3D

    const targetCamera = is3D ? camera3D : camera2D
    // const targetMorph = is3D ? 1 : 0  // Reserved for future morph animations

    // Disable controls during transition
    if (controlsRef.current) {
      controlsRef.current.enabled = false
    }

    // Create animation timeline
    const tl = gsap.timeline({
      onComplete: () => {
        if (controlsRef.current) {
          controlsRef.current.enabled = is3D
        }
        // For 2D mode, ensure camera is perfectly overhead with narrow FOV
        if (!is3D) {
          camera.position.set(0, 50, 0)
          camera.rotation.set(-Math.PI / 2, 0, 0)
          ;(camera as THREE.PerspectiveCamera).fov = camera2D.fov
          camera.updateProjectionMatrix()
        }
        onTransitionComplete()
      }
    })

    if (!is3D) {
      const perspCam = camera as THREE.PerspectiveCamera

      // Going from 3D to 2D: everything animates together
      tl.to(camera.position, {
        x: 0,
        y: 50,
        z: 0,
        duration: 1.5,
        ease: 'power2.inOut'
      }, 0)

      tl.to(camera.rotation, {
        x: -Math.PI / 2,
        y: 0,
        z: 0,
        duration: 1.5,
        ease: 'power2.inOut'
      }, 0)

      tl.to(perspCam, {
        fov: camera2D.fov,
        duration: 1.5,
        ease: 'power2.inOut',
        onUpdate: () => perspCam.updateProjectionMatrix()
      }, 0)

      tl.to(morphProgressRef.current, {
        value: 0,
        duration: 1.5,
        ease: 'power2.inOut'
      }, 0)

      if (controlsRef.current) {
        tl.to(controlsRef.current.target, {
          x: 0,
          y: 0,
          z: 0,
          duration: 1.5,
          ease: 'power2.inOut'
        }, 0)
      }
    } else {
      const perspCam = camera as THREE.PerspectiveCamera

      // Going from 2D to 3D: Restore FOV, move camera, morph pieces
      tl.to(perspCam, {
        fov: 45,
        duration: 1.5,
        ease: 'power2.inOut',
        onUpdate: () => perspCam.updateProjectionMatrix()
      }, 0)

      tl.to(camera.position, {
        x: targetCamera.position.x,
        y: targetCamera.position.y,
        z: targetCamera.position.z,
        duration: 1.5,
        ease: 'power2.inOut'
      }, 0)

      tl.to(camera.rotation, {
        x: -0.6,
        y: 0,
        z: 0,
        duration: 1.5,
        ease: 'power2.inOut'
      }, 0)

      tl.to(morphProgressRef.current, {
        value: 1,
        duration: 1.5,
        ease: 'power2.inOut'
      }, 0)
    }

    // Update controls target
    if (controlsRef.current) {
      tl.to(controlsRef.current.target, {
        x: targetCamera.target.x,
        y: targetCamera.target.y,
        z: targetCamera.target.z,
        duration: 1.5,
        ease: 'power2.inOut'
      }, 0)
    }

  }, [is3D, camera, camera3D, camera2D, onTransitionComplete])

  // Deselect lifted piece on Escape key
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && gameState.selectedPieceId) {
        gameState.selectPiece(null)
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [gameState.selectedPieceId, gameState.selectPiece])

  useFrame(() => {
    if (is3D) {
      if (controlsRef.current) {
        controlsRef.current.update()
      }
    } else if (!isTransitioning) {
      // Enforce exact overhead camera in 2D to prevent OrbitControls drift
      camera.position.set(0, 50, 0)
      camera.rotation.set(-Math.PI / 2, 0, 0)
    }
  })

  const lighting = settings.lighting

  return (
    <>
      {/* Environment map for reflections - off for soft/dramatic to isolate */}
      {lighting !== 'soft' && (
        <Environment preset="studio" environmentIntensity={lighting === 'dramatic' ? 0.02 : 0.1} />
      )}

      {/* === STANDARD LIGHTING === */}
      {lighting === 'standard' && (
        <>
          <hemisphereLight args={['#b1c4e0', '#8b7355', 0.4]} />
          <directionalLight
            position={[3, 14, 6]}
            intensity={1.0}
            castShadow
            shadow-mapSize-width={2048}
            shadow-mapSize-height={2048}
            shadow-camera-far={50}
            shadow-camera-left={-10}
            shadow-camera-right={10}
            shadow-camera-top={10}
            shadow-camera-bottom={-10}
          />
          <directionalLight position={[8, 10, 2]} intensity={0.4} />
          <directionalLight position={[-6, 8, 3]} intensity={0.3} />
          <pointLight position={[0, 6, -8]} intensity={0.3} color="#4a90d9" />
        </>
      )}

      {/* === SOFT DIFFUSE — no directional lights, hemisphere only === */}
      {lighting === 'soft' && (
        <>
          <hemisphereLight args={['#d4dce8', '#a89880', 0.9]} />
          <ambientLight intensity={0.3} />
        </>
      )}

      {/* === SINGLE OVERHEAD — straight down === */}
      {lighting === 'overhead' && (
        <>
          <hemisphereLight args={['#b1c4e0', '#8b7355', 0.2]} />
          <directionalLight
            position={[0, 20, 0]}
            intensity={1.2}
            castShadow
            shadow-mapSize-width={2048}
            shadow-mapSize-height={2048}
            shadow-camera-far={50}
            shadow-camera-left={-10}
            shadow-camera-right={10}
            shadow-camera-top={10}
            shadow-camera-bottom={-10}
          />
        </>
      )}

      {/* === FRONT LIT — light from player's perspective === */}
      {lighting === 'front' && (
        <>
          <hemisphereLight args={['#b1c4e0', '#8b7355', 0.3]} />
          <directionalLight
            position={[0, 8, 14]}
            intensity={1.0}
            castShadow
            shadow-mapSize-width={2048}
            shadow-mapSize-height={2048}
            shadow-camera-far={50}
            shadow-camera-left={-10}
            shadow-camera-right={10}
            shadow-camera-top={10}
            shadow-camera-bottom={-10}
          />
          <directionalLight position={[5, 6, 10]} intensity={0.3} />
        </>
      )}

      {/* === DRAMATIC SIDE — strong single side light for specular testing === */}
      {lighting === 'dramatic' && (
        <>
          <hemisphereLight args={['#b1c4e0', '#8b7355', 0.15]} />
          <directionalLight
            position={[-10, 6, 2]}
            intensity={1.5}
            castShadow
            shadow-mapSize-width={2048}
            shadow-mapSize-height={2048}
            shadow-camera-far={50}
            shadow-camera-left={-10}
            shadow-camera-right={10}
            shadow-camera-top={10}
            shadow-camera-bottom={-10}
          />
        </>
      )}

      <group ref={groupRef} rotation-y={boardRotation}>
        <ChessBoard
          morphProgress={morphProgressRef}
          gameState={gameState}
          boardMaterial={settings.boardMaterial}
          onlineContext={onlineContext}
        />
        <ChessPieces
          morphProgress={morphProgressRef}
          gameState={gameState}
          is3D={is3D}
          onlineContext={onlineContext}
          pieceModel={settings.pieceModel}
          pieceMaterial={settings.pieceMaterial}
        />
      </group>

      <OrbitControls
        ref={controlsRef}
        enablePan={false}
        minDistance={8}
        maxDistance={25}
        minPolarAngle={Math.PI / 8}
        maxPolarAngle={Math.PI / 2.2}
        enabled={is3D && !isTransitioning}
      />
    </>
  )
}
