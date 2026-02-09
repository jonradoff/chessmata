import { useRef, useEffect, useMemo } from 'react'
import { useThree, useFrame } from '@react-three/fiber'
import { OrbitControls } from '@react-three/drei'
import * as THREE from 'three'
import gsap from 'gsap'
import { ChessBoard } from './ChessBoard'
import { ChessPieces } from './ChessPieces'
import type { useGameState } from '../hooks/useGameState'
import type { Settings } from '../hooks/useSettings'
import type { MakeMoveResponse } from '../api/gameApi'
import type { PieceType } from '../utils/chessRules'

export interface OnlineContext {
  sessionId: string | null
  playerId: string | null
  playerColor: 'white' | 'black' | null
  currentTurn: 'white' | 'black' | null
  gameStatus: 'waiting' | 'active' | 'complete' | null
  makeMove: (from: string, to: string, promotion?: string) => Promise<MakeMoveResponse>
  onInvalidMove?: (info: { reason: string; pieceType: PieceType }) => void
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

  // Camera positions
  const camera3D = useMemo(() => ({ position: new THREE.Vector3(0, 8, 12), target: new THREE.Vector3(0, 0, 0) }), [])
  const camera2D = useMemo(() => ({ position: new THREE.Vector3(0, 28, 0), target: new THREE.Vector3(0, 0, 0) }), []) // Higher camera reduces perspective distortion

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
        // For 2D mode, ensure camera is perfectly overhead
        if (!is3D) {
          camera.position.set(0, 28, 0)
          camera.rotation.set(-Math.PI / 2, 0, 0)
          camera.updateProjectionMatrix()
        }
        onTransitionComplete()
      }
    })

    if (!is3D) {
      // Going from 3D to 2D: First move camera overhead, THEN morph
      // Phase 1: Move camera to overhead view of 3D board (0.8s)
      tl.to(camera.position, {
        x: 0,
        y: 16,
        z: 0.1, // Slight offset to avoid gimbal lock
        duration: 0.8,
        ease: 'power2.inOut'
      }, 0)

      tl.to(camera.rotation, {
        x: -Math.PI / 2,
        y: 0,
        z: 0,
        duration: 0.8,
        ease: 'power2.inOut'
      }, 0)

      if (controlsRef.current) {
        tl.to(controlsRef.current.target, {
          x: 0,
          y: 0,
          z: 0,
          duration: 0.8,
          ease: 'power2.inOut'
        }, 0)
      }

      // Phase 2: Morph to 2D and zoom out (1.2s, starts at 0.7s for overlap)
      tl.to(morphProgressRef.current, {
        value: 0,
        duration: 1.2,
        ease: 'power2.inOut'
      }, 0.7)

      tl.to(camera.position, {
        y: 28,
        z: 0,
        duration: 1.0,
        ease: 'power2.inOut'
      }, 0.9)
    } else {
      // Going from 2D to 3D: Morph and camera move together
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

  useFrame(() => {
    if (controlsRef.current) {
      controlsRef.current.update()
    }
  })

  return (
    <>
      {/* Ambient light for overall illumination */}
      <ambientLight intensity={0.5} />

      {/* Main directional light (sun-like) from front-right */}
      <directionalLight
        position={[8, 12, 10]}
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

      {/* Front fill light - illuminates black pieces facing the player */}
      <directionalLight
        position={[0, 8, 12]}
        intensity={0.8}
        color="#ffffff"
      />

      {/* Fill light from left side */}
      <directionalLight
        position={[-8, 6, 5]}
        intensity={0.4}
      />

      {/* Back rim light for dramatic effect on white pieces */}
      <pointLight
        position={[0, 8, -8]}
        intensity={0.4}
        color="#4a90d9"
      />

      <group ref={groupRef}>
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
