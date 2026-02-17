import type { ReactNode } from 'react'
import { useRef, useMemo, useState, useEffect } from 'react'
import { useFrame } from '@react-three/fiber'
import type { ThreeEvent } from '@react-three/fiber'
import { useGLTF } from '@react-three/drei'
import * as THREE from 'three'
import { boardToWorld } from '../hooks/useGameState'
import type { PieceState, useGameState, MoveArrow } from '../hooks/useGameState'
import type { OnlineContext } from './ChessScene'
import { canSelectPiece, isValidMove, toAlgebraic, isInCheck } from '../utils/chessRules'
import { parseFENState } from '../utils/fenParser'
import type { PieceModel, PieceMaterial } from '../hooks/useSettings'

interface ChessPiecesProps {
  morphProgress: React.MutableRefObject<{ value: number }>
  gameState: ReturnType<typeof useGameState>
  is3D: boolean
  onlineContext?: OnlineContext
  pieceModel?: PieceModel
  pieceMaterial?: PieceMaterial
}

type PieceType = 'king' | 'queen' | 'rook' | 'bishop' | 'knight' | 'pawn'

// Build board from FEN state for move validation
function boardFromFEN(fen: string) {
  const state = parseFENState(fen)
  return {
    pieces: state.pieces.map(p => ({
      type: p.type,
      isWhite: p.isWhite,
      file: p.file,
      rank: p.rank,
    })),
    whiteToMove: state.whiteToMove,
    castlingRights: state.castlingRights,
    enPassantSquare: state.enPassantSquare,
  }
}

export function ChessPieces({ morphProgress, gameState, is3D, onlineContext, pieceModel = 'basic', pieceMaterial = 'simple' }: ChessPiecesProps) {
  const { pieces, selectedPieceId, hoverSquare } = gameState

  // Determine if either king is in check
  const board = useMemo(() => boardFromFEN(gameState.boardFEN), [gameState.boardFEN])
  const whiteKingInCheck = useMemo(() => isInCheck(board, true), [board])
  const blackKingInCheck = useMemo(() => isInCheck(board, false), [board])

  return (
    <group>
      {pieces.map((piece) => {
        // Determine if this piece is a king in check
        const isKingInCheck = piece.type === 'king' &&
          ((piece.isWhite && whiteKingInCheck) || (!piece.isWhite && blackKingInCheck))

        return (
          <ChessPiece
            key={piece.id}
            piece={piece}
            morphProgress={morphProgress}
            gameState={gameState}
            is3D={is3D}
            isSelected={selectedPieceId === piece.id}
            isInCheck={isKingInCheck}
            onlineContext={onlineContext}
            pieceModel={pieceModel}
            pieceMaterial={pieceMaterial}
          />
        )
      })}

      {/* Drop shadow indicator for selected piece in 3D mode */}
      {is3D && selectedPieceId && hoverSquare && (
        <DropShadow file={hoverSquare.file} rank={hoverSquare.rank} />
      )}

      {/* Arrow showing opponent's last move */}
      {gameState.lastMoveArrow && (
        <LastMoveArrow arrow={gameState.lastMoveArrow} />
      )}
    </group>
  )
}

interface ChessPieceProps {
  piece: PieceState
  morphProgress: React.MutableRefObject<{ value: number }>
  gameState: ReturnType<typeof useGameState>
  is3D: boolean
  isSelected: boolean
  isInCheck?: boolean
  onlineContext?: OnlineContext
  pieceModel?: PieceModel
  pieceMaterial?: PieceMaterial
}

function ChessPiece({ piece, morphProgress, gameState, is3D, isSelected, isInCheck: pieceInCheck, onlineContext, pieceModel = 'basic', pieceMaterial = 'simple' }: ChessPieceProps) {
  const groupRef = useRef<THREE.Group>(null)
  const piece3DRef = useRef<THREE.Group>(null)
  const piece2DRef = useRef<THREE.Group>(null)

  // Animation state for lifted piece
  const liftProgress = useRef(0)
  const targetLift = isSelected ? 1 : 0

  // Calculate base position from board coordinates
  const basePosition = useMemo(() => boardToWorld(piece.file, piece.rank), [piece.file, piece.rank])

  useFrame((state, delta) => {
    if (groupRef.current && piece3DRef.current && piece2DRef.current) {
      const progress = morphProgress.current.value

      // Animate lift smoothly
      const liftSpeed = 8
      if (liftProgress.current < targetLift) {
        liftProgress.current = Math.min(liftProgress.current + delta * liftSpeed, targetLift)
      } else if (liftProgress.current > targetLift) {
        liftProgress.current = Math.max(liftProgress.current - delta * liftSpeed, targetLift)
      }

      // Calculate position with lift effect
      const liftHeight = liftProgress.current * 1.5
      const bobAmount = isSelected ? Math.sin(state.clock.elapsedTime * 3) * 0.05 : 0

      groupRef.current.position.set(
        basePosition[0],
        basePosition[1] + liftHeight + bobAmount,
        basePosition[2]
      )

      // 3D piece visibility
      piece3DRef.current.visible = progress > 0.01
      piece3DRef.current.scale.setScalar(progress * (isSelected && is3D ? 1.1 : 1))
      piece3DRef.current.position.y = 0

      // 2D piece visibility
      const twoDScale = 1 - progress
      piece2DRef.current.visible = twoDScale > 0.01
      const scale2D = twoDScale * (isSelected && !is3D ? 1.15 : 1)
      piece2DRef.current.scale.set(scale2D, scale2D, 1)
      piece2DRef.current.position.y = 0.1
    }
  })

  const handleClick = async (e: ThreeEvent<MouseEvent>) => {
    e.stopPropagation()

    // Check if we're in an online game
    const isOnlineGame = onlineContext?.sessionId != null
    const playerColor = onlineContext?.playerColor
    const currentTurn = onlineContext?.currentTurn
    const gameStatus = onlineContext?.gameStatus

    // Don't allow any moves if no game session exists
    if (!isOnlineGame) {
      return
    }

    // In watch/spectator mode (no makeMove), don't allow interaction
    if (!onlineContext?.makeMove) {
      return
    }

    // If game is waiting for opponent or complete, don't allow moves
    if (gameStatus === 'waiting' || gameStatus === 'complete') {
      return
    }

    if (isSelected) {
      // The lifted piece can intercept clicks meant for the board square
      // beneath it. If there's a hover square showing, execute the move there.
      if (is3D && gameState.hoverSquare) {
        const board = boardFromFEN(gameState.boardFEN)
        const from = { file: piece.file, rank: piece.rank }
        const to = { file: gameState.hoverSquare.file, rank: gameState.hoverSquare.rank }
        const validation = isValidMove(board, from, to)

        if (validation.valid) {
          if (piece.type === 'pawn' && (to.rank === 7 || to.rank === 0)) {
            const fromNotation = toAlgebraic(from.file, from.rank)
            const toNotation = toAlgebraic(to.file, to.rank)
            if (onlineContext?.onPendingPromotion) {
              onlineContext.onPendingPromotion({
                from: fromNotation,
                to: toNotation,
                pieceId: piece.id,
                isWhite: piece.isWhite,
                toFile: to.file,
                toRank: to.rank,
              })
            }
            gameState.selectPiece(null)
            return
          }

          if (isOnlineGame && onlineContext?.makeMove) {
            gameState.movePiece(piece.id, to.file, to.rank)
            const fromNotation = toAlgebraic(from.file, from.rank)
            const toNotation = toAlgebraic(to.file, to.rank)
            try {
              const response = await onlineContext.makeMove(fromNotation, toNotation)
              if (!response.success) {
                console.error('Move rejected by server:', response.error)
              }
            } catch (err) {
              console.error('Failed to make move:', err)
            }
          } else {
            gameState.movePiece(piece.id, to.file, to.rank)
          }
          return
        }
      }
      gameState.selectPiece(null)
    } else if (gameState.selectedPieceId) {
      // Trying to move selected piece to this square (capture or move)
      const selectedPiece = gameState.pieces.find(p => p.id === gameState.selectedPieceId)
      if (!selectedPiece) return

      // Check if we're trying to capture our own piece
      if (piece.isWhite === selectedPiece.isWhite) {
        // Just select this piece instead
        if (canSelectPiece(piece, playerColor || null, currentTurn || null)) {
          gameState.selectPiece(piece.id)
        }
        return
      }

      // Validate the move
      const board = boardFromFEN(gameState.boardFEN)
      const from = { file: selectedPiece.file, rank: selectedPiece.rank }
      const to = { file: piece.file, rank: piece.rank }
      const validation = isValidMove(board, from, to)

      if (!validation.valid) {
        // Show invalid move modal
        if (onlineContext?.onInvalidMove && validation.reason && validation.pieceType) {
          onlineContext.onInvalidMove({
            reason: validation.reason,
            pieceType: validation.pieceType
          })
        }
        gameState.selectPiece(null)
        return
      }

      // Detect pawn promotion
      if (selectedPiece.type === 'pawn' && (piece.rank === 7 || piece.rank === 0)) {
        const fromNotation = toAlgebraic(from.file, from.rank)
        const toNotation = toAlgebraic(to.file, to.rank)
        if (onlineContext?.onPendingPromotion) {
          onlineContext.onPendingPromotion({
            from: fromNotation,
            to: toNotation,
            pieceId: gameState.selectedPieceId,
            isWhite: selectedPiece.isWhite,
            toFile: piece.file,
            toRank: piece.rank,
          })
        }
        gameState.selectPiece(null)
        return
      }

      // If online game, optimistically update locally then send to server
      if (isOnlineGame && onlineContext?.makeMove) {
        // Optimistically move the piece immediately for instant feedback
        gameState.movePiece(gameState.selectedPieceId, piece.file, piece.rank)

        const fromNotation = toAlgebraic(from.file, from.rank)
        const toNotation = toAlgebraic(to.file, to.rank)
        try {
          const response = await onlineContext.makeMove(fromNotation, toNotation)
          // If move was rejected, board will sync back from server
          if (!response.success) {
            console.error('Move rejected by server:', response.error)
          }
        } catch (err) {
          console.error('Failed to make move:', err)
          // Board will sync back from server on error
        }
      } else {
        // Local game - update state directly
        gameState.movePiece(gameState.selectedPieceId, piece.file, piece.rank)
      }
    } else {
      // Trying to select this piece
      if (!canSelectPiece(piece, playerColor || null, currentTurn || null)) {
        return // Can't select opponent's piece or not your turn
      }
      gameState.selectPiece(piece.id)
    }
  }

  return (
    <group
      ref={groupRef}
      position={basePosition}
      onClick={handleClick}
    >
      <group ref={piece3DRef}>
        <Piece3D type={piece.type as PieceType} isWhite={piece.isWhite} isSelected={isSelected && is3D} isInCheck={pieceInCheck} pieceModel={pieceModel} pieceMaterial={pieceMaterial} />
      </group>
      <group ref={piece2DRef}>
        <Piece2D type={piece.type as PieceType} isWhite={piece.isWhite} isSelected={isSelected && !is3D} isInCheck={pieceInCheck} boardYRotation={onlineContext?.playerColor === 'white' ? Math.PI : 0} />
      </group>
      {isSelected && is3D && <SelectionRing />}
      {pieceInCheck && is3D && <CheckRing />}
    </group>
  )
}

function DropShadow({ file, rank }: { file: number; rank: number }) {
  const position = boardToWorld(file, rank)
  const meshRef = useRef<THREE.Mesh>(null)

  useFrame((state) => {
    if (meshRef.current) {
      const opacity = 0.3 + Math.sin(state.clock.elapsedTime * 4) * 0.1
      ;(meshRef.current.material as THREE.MeshBasicMaterial).opacity = opacity
    }
  })

  return (
    <mesh ref={meshRef} position={[position[0], 0.06, position[2]]} rotation={[-Math.PI / 2, 0, 0]}>
      <circleGeometry args={[0.4, 32]} />
      <meshBasicMaterial color="#000000" transparent opacity={0.3} />
    </mesh>
  )
}

function LastMoveArrow({ arrow }: { arrow: MoveArrow }) {
  const meshRef = useRef<THREE.Mesh>(null)

  const geometry = useMemo(() => {
    const fromPos = boardToWorld(arrow.from.file, arrow.from.rank)
    const toPos = boardToWorld(arrow.to.file, arrow.to.rank)

    const dx = toPos[0] - fromPos[0]
    const dz = toPos[2] - fromPos[2]
    const length = Math.sqrt(dx * dx + dz * dz)
    if (length < 0.01) return null

    // Direction and perpendicular vectors
    const nx = dx / length
    const nz = dz / length
    const px = -nz
    const pz = nx

    const shaftWidth = 0.1
    const headWidth = 0.28
    const headLength = 0.35
    const shaftLen = length - headLength
    const y = 0.08

    // Start slightly inset from square center
    const inset = 0.15
    const sx = fromPos[0] + nx * inset
    const sz = fromPos[2] + nz * inset
    const ex = fromPos[0] + nx * (inset + shaftLen)
    const ez = fromPos[2] + nz * (inset + shaftLen)
    const tx = toPos[0] - nx * inset
    const tz = toPos[2] - nz * inset

    // 9 vertices: shaft (2 triangles = 6 verts) + head (1 triangle = 3 verts)
    const vertices = new Float32Array([
      // Shaft quad
      sx + px * shaftWidth, y, sz + pz * shaftWidth,
      sx - px * shaftWidth, y, sz - pz * shaftWidth,
      ex + px * shaftWidth, y, ez + pz * shaftWidth,

      sx - px * shaftWidth, y, sz - pz * shaftWidth,
      ex - px * shaftWidth, y, ez - pz * shaftWidth,
      ex + px * shaftWidth, y, ez + pz * shaftWidth,

      // Arrowhead triangle
      ex + px * headWidth, y, ez + pz * headWidth,
      ex - px * headWidth, y, ez - pz * headWidth,
      tx, y, tz,
    ])

    const geo = new THREE.BufferGeometry()
    geo.setAttribute('position', new THREE.BufferAttribute(vertices, 3))
    geo.computeVertexNormals()
    return geo
  }, [arrow.from.file, arrow.from.rank, arrow.to.file, arrow.to.rank])

  useFrame((state) => {
    if (meshRef.current) {
      const opacity = 0.55 + Math.sin(state.clock.elapsedTime * 2.5) * 0.1
      ;(meshRef.current.material as THREE.MeshBasicMaterial).opacity = opacity
    }
  })

  if (!geometry) return null

  return (
    <mesh ref={meshRef} geometry={geometry}>
      <meshBasicMaterial
        color="#f0b830"
        transparent
        opacity={0.55}
        side={THREE.DoubleSide}
        depthWrite={false}
      />
    </mesh>
  )
}

function SelectionRing() {
  const ringRef = useRef<THREE.Mesh>(null)
  useFrame((state) => {
    if (ringRef.current) {
      ringRef.current.rotation.z = state.clock.elapsedTime * 2
    }
  })
  return (
    <mesh ref={ringRef} position={[0, 0.02, 0]} rotation={[-Math.PI / 2, 0, 0]}>
      <ringGeometry args={[0.42, 0.48, 32]} />
      <meshBasicMaterial color="#4CAF50" transparent opacity={0.7} side={THREE.DoubleSide} />
    </mesh>
  )
}

function CheckRing() {
  const ringRef = useRef<THREE.Mesh>(null)
  useFrame((state) => {
    if (ringRef.current) {
      // Pulsing red glow effect
      const pulse = 0.6 + Math.sin(state.clock.elapsedTime * 4) * 0.3
      ;(ringRef.current.material as THREE.MeshBasicMaterial).opacity = pulse
    }
  })
  return (
    <mesh ref={ringRef} position={[0, 0.02, 0]} rotation={[-Math.PI / 2, 0, 0]}>
      <ringGeometry args={[0.45, 0.55, 32]} />
      <meshBasicMaterial color="#ff3333" transparent opacity={0.8} side={THREE.DoubleSide} />
    </mesh>
  )
}

interface PieceRenderProps {
  type: PieceType
  isWhite: boolean
  isSelected?: boolean
  isInCheck?: boolean
  pieceModel?: PieceModel
  pieceMaterial?: PieceMaterial
  boardYRotation?: number
}

// Material cache: keyed by "pieceMaterial-isWhite" to avoid recreating materials every frame.
// Emissive properties are updated mutably on the cached material per render.
const _materialCache = new Map<string, THREE.Material>()
const _woodTextureCache = new Map<string, THREE.CanvasTexture>()

// Creates a special material for Crystal, Chrome, or Wood piece modes.
// Returns null for other modes (caller uses its default behavior).
// Materials are cached and reused; only emissive properties are updated per call.
function createSpecialMaterial(
  pieceMaterial: PieceMaterial,
  isWhite: boolean,
  isSelected: boolean | undefined,
  isInCheck: boolean | undefined
): THREE.Material | null {
  if (pieceMaterial !== 'crystal' && pieceMaterial !== 'chrome' && pieceMaterial !== 'wood') return null

  const checkEmissive = '#ff3333'
  const selectedEmissive = isWhite ? '#88ff88' : '#44aa44'
  let emissive = '#000000'
  let emissiveIntensity = 0
  if (isInCheck) { emissive = checkEmissive; emissiveIntensity = 0.5 }
  else if (isSelected) { emissive = selectedEmissive; emissiveIntensity = 0.2 }

  const cacheKey = `${pieceMaterial}-${isWhite}`
  let mat = _materialCache.get(cacheKey)

  if (!mat) {
    if (pieceMaterial === 'crystal') {
      mat = new THREE.MeshPhysicalMaterial({
        color: isWhite ? '#e8f4ff' : '#2a1f3d',
        transmission: isWhite ? 0.92 : 0.65,
        transparent: true,
        opacity: 1,
        ior: 1.5,
        thickness: isWhite ? 0.5 : 0.8,
        roughness: isWhite ? 0.05 : 0.12,
        metalness: 0.0,
        envMapIntensity: 1.0,
        emissive: new THREE.Color(emissive),
        emissiveIntensity,
      })
    } else if (pieceMaterial === 'chrome') {
      mat = new THREE.MeshStandardMaterial({
        color: isWhite ? '#f8f8f8' : '#1a1a22',
        metalness: isWhite ? 1.0 : 0.75,
        roughness: isWhite ? 0.15 : 0.2,
        envMapIntensity: isWhite ? 2.0 : 0.8,
        emissive: new THREE.Color(emissive),
        emissiveIntensity,
      })
    } else {
      // Wood: use cached texture
      const woodTexture = getCachedWoodTexture(isWhite)
      mat = new THREE.MeshStandardMaterial({
        color: '#ffffff',
        map: woodTexture,
        metalness: 0.0,
        roughness: isWhite ? 0.65 : 0.55,
        envMapIntensity: 0.1,
        emissive: new THREE.Color(emissive),
        emissiveIntensity,
      })
    }
    _materialCache.set(cacheKey, mat)
  }

  // Update emissive properties on the cached material (cheap mutable update)
  if ('emissive' in mat) {
    const stdMat = mat as THREE.MeshStandardMaterial
    stdMat.emissive.set(emissive)
    stdMat.emissiveIntensity = emissiveIntensity
    // Chrome white has a special default emissive when not selected/in-check
    if (pieceMaterial === 'chrome' && isWhite && !isInCheck && !isSelected) {
      stdMat.emissive.set('#444444')
      stdMat.emissiveIntensity = 0.08
    }
  }

  return mat
}

// Returns a cached wood texture, creating it only once per color
function getCachedWoodTexture(isWhite: boolean): THREE.CanvasTexture {
  const key = isWhite ? 'white' : 'black'
  let tex = _woodTextureCache.get(key)
  if (!tex) {
    tex = createPieceWoodTexture(isWhite)
    _woodTextureCache.set(key, tex)
  }
  return tex
}

// Procedural wood grain texture for piece material — 512px for visible detail
function createPieceWoodTexture(isWhite: boolean): THREE.CanvasTexture {
  const canvas = document.createElement('canvas')
  canvas.width = 512
  canvas.height = 512
  const ctx = canvas.getContext('2d')!

  if (isWhite) {
    // Oak: warm honey-tan base with prominent darker grain lines and large knots
    ctx.fillStyle = '#d4a96a'
    ctx.fillRect(0, 0, 512, 512)

    // Broad growth-ring bands for variation
    for (let b = 0; b < 8; b++) {
      const y = Math.random() * 512
      const h = 10 + Math.random() * 25
      ctx.fillStyle = `rgba(190, 140, 70, ${0.12 + Math.random() * 0.15})`
      ctx.fillRect(0, y, 512, h)
    }

    // Grain lines — strong and visible
    for (let i = 0; i < 55; i++) {
      const y = Math.random() * 512
      const waveAmp = 3 + Math.random() * 6
      ctx.beginPath()
      ctx.strokeStyle = `rgba(145, 95, 35, ${0.3 + Math.random() * 0.35})`
      ctx.lineWidth = 1 + Math.random() * 3
      for (let x = 0; x < 512; x += 3) {
        const yOff = y + Math.sin(x * 0.01 + i) * waveAmp + Math.sin(x * 0.03) * waveAmp * 0.5
        if (x === 0) ctx.moveTo(x, yOff)
        else ctx.lineTo(x, yOff)
      }
      ctx.stroke()
    }

    // Large, visible knots / whorls
    for (let k = 0; k < 5; k++) {
      const cx = 60 + Math.random() * 390
      const cy = 60 + Math.random() * 390
      // Dark center spot
      ctx.beginPath()
      ctx.fillStyle = `rgba(120, 75, 25, ${0.35 + Math.random() * 0.2})`
      ctx.ellipse(cx, cy, 4 + Math.random() * 5, 3 + Math.random() * 4, Math.random() * Math.PI, 0, Math.PI * 2)
      ctx.fill()
      // Concentric rings
      const maxR = 20 + Math.random() * 30
      const tilt = (Math.random() - 0.5) * 0.8
      for (let r = 5; r < maxR; r += 2 + Math.random() * 2) {
        ctx.beginPath()
        ctx.strokeStyle = `rgba(130, 80, 25, ${0.2 + Math.random() * 0.25})`
        ctx.lineWidth = 1 + Math.random() * 1.5
        ctx.ellipse(cx, cy, r * (0.7 + Math.random() * 0.6), r, tilt, 0, Math.PI * 2)
        ctx.stroke()
      }
    }
  } else {
    // Dark Walnut: rich dark brown base with lighter grain streaks and knots
    ctx.fillStyle = '#3d2b1f'
    ctx.fillRect(0, 0, 512, 512)

    // Broad lighter bands
    for (let b = 0; b < 6; b++) {
      const y = Math.random() * 512
      const h = 10 + Math.random() * 20
      ctx.fillStyle = `rgba(75, 55, 35, ${0.15 + Math.random() * 0.15})`
      ctx.fillRect(0, y, 512, h)
    }

    // Grain lines — lighter brown streaks, pronounced
    for (let i = 0; i < 50; i++) {
      const y = Math.random() * 512
      const waveAmp = 3 + Math.random() * 6
      ctx.beginPath()
      ctx.strokeStyle = `rgba(95, 70, 45, ${0.35 + Math.random() * 0.4})`
      ctx.lineWidth = 1 + Math.random() * 3
      for (let x = 0; x < 512; x += 3) {
        const yOff = y + Math.sin(x * 0.009 + i * 0.7) * waveAmp + Math.sin(x * 0.025) * waveAmp * 0.6
        if (x === 0) ctx.moveTo(x, yOff)
        else ctx.lineTo(x, yOff)
      }
      ctx.stroke()
    }

    // Large, visible knots
    for (let k = 0; k < 4; k++) {
      const cx = 60 + Math.random() * 390
      const cy = 60 + Math.random() * 390
      // Dark center
      ctx.beginPath()
      ctx.fillStyle = `rgba(25, 15, 8, ${0.4 + Math.random() * 0.25})`
      ctx.ellipse(cx, cy, 4 + Math.random() * 4, 3 + Math.random() * 3, Math.random() * Math.PI, 0, Math.PI * 2)
      ctx.fill()
      // Concentric rings
      const maxR = 18 + Math.random() * 25
      const tilt = (Math.random() - 0.5) * 0.7
      for (let r = 5; r < maxR; r += 2 + Math.random() * 2) {
        ctx.beginPath()
        ctx.strokeStyle = `rgba(60, 40, 20, ${0.25 + Math.random() * 0.3})`
        ctx.lineWidth = 1 + Math.random() * 1.5
        ctx.ellipse(cx, cy, r * (0.7 + Math.random() * 0.6), r, tilt, 0, Math.PI * 2)
        ctx.stroke()
      }
    }
  }

  const texture = new THREE.CanvasTexture(canvas)
  texture.wrapS = THREE.RepeatWrapping
  texture.wrapT = THREE.RepeatWrapping
  return texture
}

// Generate box-projection UVs for meshes that lack them (e.g. Meshy, Fantasy STL-based models).
// Uses normal-weighted triplanar projection so the texture wraps sensibly around the piece.
// No-op if the geometry already has UV coordinates.
function ensureMeshUVs(geometry: THREE.BufferGeometry): void {
  if (geometry.attributes.uv) return

  const positions = geometry.attributes.position
  const count = positions.count
  const uvs = new Float32Array(count * 2)

  geometry.computeBoundingBox()
  const box = geometry.boundingBox!
  const size = new THREE.Vector3()
  box.getSize(size)
  // Avoid division by zero on degenerate axes
  const sx = size.x || 1
  const sy = size.y || 1
  const sz = size.z || 1

  const hasNormals = !!geometry.attributes.normal

  for (let i = 0; i < count; i++) {
    const x = positions.getX(i)
    const y = positions.getY(i)
    const z = positions.getZ(i)

    if (hasNormals) {
      const nx = Math.abs(geometry.attributes.normal.getX(i))
      const ny = Math.abs(geometry.attributes.normal.getY(i))
      const nz = Math.abs(geometry.attributes.normal.getZ(i))

      if (ny >= nx && ny >= nz) {
        // Top/bottom — project from Y axis
        uvs[i * 2] = (x - box.min.x) / sx
        uvs[i * 2 + 1] = (z - box.min.z) / sz
      } else if (nx >= nz) {
        // Side — project from X axis
        uvs[i * 2] = (z - box.min.z) / sz
        uvs[i * 2 + 1] = (y - box.min.y) / sy
      } else {
        // Front/back — project from Z axis
        uvs[i * 2] = (x - box.min.x) / sx
        uvs[i * 2 + 1] = (y - box.min.y) / sy
      }
    } else {
      // Fallback: simple Y-axis projection
      uvs[i * 2] = (x - box.min.x) / sx
      uvs[i * 2 + 1] = (y - box.min.y) / sy
    }
  }

  geometry.setAttribute('uv', new THREE.BufferAttribute(uvs, 2))
}

// Procedural stone texture with striations and grain — for "realistic" material on models
// that lack built-in textures (Meshy, Fantasy, etc.)
function createStoneTexture(isWhite: boolean): THREE.CanvasTexture {
  const canvas = document.createElement('canvas')
  canvas.width = 512
  canvas.height = 512
  const ctx = canvas.getContext('2d')!

  if (isWhite) {
    // Warm white stone — travertine / sandstone appearance
    ctx.fillStyle = '#ede7db'
    ctx.fillRect(0, 0, 512, 512)

    // Broad sedimentary bands
    for (let b = 0; b < 6; b++) {
      const y = 30 + Math.random() * 450
      const h = 15 + Math.random() * 30
      ctx.fillStyle = `rgba(200, 185, 160, ${0.15 + Math.random() * 0.15})`
      ctx.fillRect(0, y, 512, h)
    }

    // Horizontal striations — layered stone grain
    for (let i = 0; i < 60; i++) {
      const y = Math.random() * 512
      const waveAmp = 2 + Math.random() * 5
      ctx.beginPath()
      ctx.strokeStyle = `rgba(175, 155, 125, ${0.2 + Math.random() * 0.35})`
      ctx.lineWidth = 0.8 + Math.random() * 2.5
      for (let x = 0; x < 512; x += 3) {
        const yOff = y + Math.sin(x * 0.008 + i) * waveAmp + Math.sin(x * 0.025) * waveAmp * 0.4
        if (x === 0) ctx.moveTo(x, yOff)
        else ctx.lineTo(x, yOff)
      }
      ctx.stroke()
    }

    // Veining — darker cracks running through the stone
    for (let v = 0; v < 8; v++) {
      ctx.beginPath()
      ctx.strokeStyle = `rgba(150, 130, 100, ${0.15 + Math.random() * 0.25})`
      ctx.lineWidth = 0.5 + Math.random() * 1.5
      let x = Math.random() * 512
      let y = Math.random() * 512
      ctx.moveTo(x, y)
      for (let j = 0; j < 10; j++) {
        const cx = x + (Math.random() - 0.5) * 80
        const cy = y + (Math.random() - 0.3) * 50
        x += (Math.random() - 0.5) * 70
        y += (Math.random() - 0.3) * 40
        ctx.quadraticCurveTo(cx, cy, x, y)
      }
      ctx.stroke()
    }

    // Grain / speckle
    const imageData = ctx.getImageData(0, 0, 512, 512)
    const data = imageData.data
    for (let i = 0; i < data.length; i += 4) {
      const noise = (Math.random() - 0.5) * 14
      data[i] = Math.max(0, Math.min(255, data[i] + noise))
      data[i + 1] = Math.max(0, Math.min(255, data[i + 1] + noise))
      data[i + 2] = Math.max(0, Math.min(255, data[i + 2] + noise))
    }
    ctx.putImageData(imageData, 0, 0)
  } else {
    // Dark stone — slate / dark granite
    ctx.fillStyle = '#2c2c2c'
    ctx.fillRect(0, 0, 512, 512)

    // Broad darker / lighter bands
    for (let b = 0; b < 5; b++) {
      const y = 30 + Math.random() * 450
      const h = 15 + Math.random() * 25
      ctx.fillStyle = `rgba(50, 48, 44, ${0.2 + Math.random() * 0.2})`
      ctx.fillRect(0, y, 512, h)
    }

    // Striations
    for (let i = 0; i < 50; i++) {
      const y = Math.random() * 512
      const waveAmp = 2 + Math.random() * 4
      ctx.beginPath()
      ctx.strokeStyle = `rgba(65, 62, 56, ${0.3 + Math.random() * 0.4})`
      ctx.lineWidth = 0.8 + Math.random() * 2
      for (let x = 0; x < 512; x += 3) {
        const yOff = y + Math.sin(x * 0.01 + i * 0.5) * waveAmp + Math.sin(x * 0.03) * waveAmp * 0.5
        if (x === 0) ctx.moveTo(x, yOff)
        else ctx.lineTo(x, yOff)
      }
      ctx.stroke()
    }

    // Subtle lighter veining
    for (let v = 0; v < 6; v++) {
      ctx.beginPath()
      ctx.strokeStyle = `rgba(85, 80, 72, ${0.2 + Math.random() * 0.3})`
      ctx.lineWidth = 0.5 + Math.random() * 1.2
      let x = Math.random() * 512
      let y = Math.random() * 512
      ctx.moveTo(x, y)
      for (let j = 0; j < 8; j++) {
        const cx = x + (Math.random() - 0.5) * 70
        const cy = y + (Math.random() - 0.3) * 45
        x += (Math.random() - 0.5) * 60
        y += (Math.random() - 0.3) * 35
        ctx.quadraticCurveTo(cx, cy, x, y)
      }
      ctx.stroke()
    }

    // Grain / speckle
    const imageData = ctx.getImageData(0, 0, 512, 512)
    const data = imageData.data
    for (let i = 0; i < data.length; i += 4) {
      const noise = (Math.random() - 0.5) * 10
      data[i] = Math.max(0, Math.min(255, data[i] + noise))
      data[i + 1] = Math.max(0, Math.min(255, data[i + 1] + noise))
      data[i + 2] = Math.max(0, Math.min(255, data[i + 2] + noise))
    }
    ctx.putImageData(imageData, 0, 0)
  }

  const texture = new THREE.CanvasTexture(canvas)
  texture.wrapS = THREE.RepeatWrapping
  texture.wrapT = THREE.RepeatWrapping
  return texture
}

function Piece3D({ type, isWhite, isSelected, isInCheck, pieceModel = 'basic', pieceMaterial = 'simple' }: PieceRenderProps) {
  // Use GLTF models for "standard" mode
  if (pieceModel === 'standard') {
    return <GLTFPiece3D type={type} isWhite={isWhite} isSelected={isSelected} isInCheck={isInCheck} pieceMaterial={pieceMaterial} />
  }

  if (pieceModel === 'detailed') {
    return <DetailedPiece3D type={type} isWhite={isWhite} isSelected={isSelected} isInCheck={isInCheck} pieceMaterial={pieceMaterial} />
  }

  if (pieceModel === 'fantasy') {
    return <FantasyPiece3D type={type} isWhite={isWhite} isSelected={isSelected} isInCheck={isInCheck} pieceMaterial={pieceMaterial} />
  }

  if (pieceModel === 'meshy') {
    return <MeshyPiece3D type={type} isWhite={isWhite} isSelected={isSelected} isInCheck={isInCheck} pieceMaterial={pieceMaterial} />
  }

  if (pieceModel === 'cubist') {
    return <CubistPiece3D type={type} isWhite={isWhite} isSelected={isSelected} isInCheck={isInCheck} pieceMaterial={pieceMaterial} />
  }

  // Basic procedural models
  const color = isWhite ? '#f5f5f5' : '#3d3d3d'
  const emissive = isWhite ? '#ffffff' : '#4a4a4a'
  const selectedEmissive = isWhite ? '#88ff88' : '#44aa44'
  const checkEmissive = '#ff3333' // Red glow for check

  // Determine emissive color based on state
  let finalEmissive = emissive
  let finalEmissiveIntensity = 0.08

  if (isInCheck) {
    finalEmissive = checkEmissive
    finalEmissiveIntensity = 0.5 // Strong red glow
  } else if (isSelected) {
    finalEmissive = selectedEmissive
    finalEmissiveIntensity = 0.2
  }

  // Check for crystal/chrome material override on basic model
  const specialMat = createSpecialMaterial(pieceMaterial, isWhite, isSelected, isInCheck)

  const material = specialMat
    ? <primitive object={specialMat} attach="material" />
    : (
    <meshStandardMaterial
      color={color}
      metalness={0.3}
      roughness={0.4}
      emissive={finalEmissive}
      emissiveIntensity={finalEmissiveIntensity}
    />
  )

  switch (type) {
    case 'king': return <King3D material={material} />
    case 'queen': return <Queen3D material={material} />
    case 'rook': return <Rook3D material={material} />
    case 'bishop': return <Bishop3D material={material} />
    case 'knight': return <Knight3D material={material} isWhite={isWhite} />
    case 'pawn': return <Pawn3D material={material} />
  }
}

function King3D({ material }: { material: ReactNode }) {
  return (
    <group>
      <mesh position={[0, 0.15, 0]} castShadow><cylinderGeometry args={[0.35, 0.4, 0.3, 32]} />{material}</mesh>
      <mesh position={[0, 0.45, 0]} castShadow><cylinderGeometry args={[0.25, 0.35, 0.3, 32]} />{material}</mesh>
      <mesh position={[0, 0.7, 0]} castShadow><cylinderGeometry args={[0.2, 0.25, 0.2, 32]} />{material}</mesh>
      <mesh position={[0, 0.85, 0]} castShadow><cylinderGeometry args={[0.25, 0.2, 0.1, 32]} />{material}</mesh>
      <mesh position={[0, 1.05, 0]} castShadow><cylinderGeometry args={[0.18, 0.22, 0.3, 32]} />{material}</mesh>
      <mesh position={[0, 1.25, 0]} castShadow><cylinderGeometry args={[0.22, 0.18, 0.1, 32]} />{material}</mesh>
      <mesh position={[0, 1.45, 0]} castShadow><boxGeometry args={[0.08, 0.3, 0.08]} />{material}</mesh>
      <mesh position={[0, 1.5, 0]} castShadow><boxGeometry args={[0.2, 0.08, 0.08]} />{material}</mesh>
    </group>
  )
}

function Queen3D({ material }: { material: ReactNode }) {
  return (
    <group>
      <mesh position={[0, 0.15, 0]} castShadow><cylinderGeometry args={[0.35, 0.4, 0.3, 32]} />{material}</mesh>
      <mesh position={[0, 0.45, 0]} castShadow><cylinderGeometry args={[0.22, 0.35, 0.3, 32]} />{material}</mesh>
      <mesh position={[0, 0.7, 0]} castShadow><cylinderGeometry args={[0.18, 0.22, 0.2, 32]} />{material}</mesh>
      <mesh position={[0, 0.85, 0]} castShadow><cylinderGeometry args={[0.22, 0.18, 0.1, 32]} />{material}</mesh>
      <mesh position={[0, 1.0, 0]} castShadow><cylinderGeometry args={[0.15, 0.2, 0.2, 32]} />{material}</mesh>
      <mesh position={[0, 1.15, 0]} castShadow><cylinderGeometry args={[0.2, 0.15, 0.1, 32]} />{material}</mesh>
      {[0,1,2,3,4,5,6,7].map(i => {
        const angle = (i/8)*Math.PI*2
        return <mesh key={i} position={[Math.cos(angle)*0.15, 1.3, Math.sin(angle)*0.15]} castShadow><sphereGeometry args={[0.05,16,16]} />{material}</mesh>
      })}
      <mesh position={[0, 1.35, 0]} castShadow><sphereGeometry args={[0.08, 16, 16]} />{material}</mesh>
    </group>
  )
}

function Rook3D({ material }: { material: ReactNode }) {
  return (
    <group>
      <mesh position={[0, 0.12, 0]} castShadow><cylinderGeometry args={[0.32, 0.38, 0.24, 32]} />{material}</mesh>
      <mesh position={[0, 0.45, 0]} castShadow><cylinderGeometry args={[0.22, 0.3, 0.4, 32]} />{material}</mesh>
      <mesh position={[0, 0.72, 0]} castShadow><cylinderGeometry args={[0.25, 0.22, 0.14, 32]} />{material}</mesh>
      <mesh position={[0, 0.85, 0]} castShadow><cylinderGeometry args={[0.28, 0.25, 0.12, 32]} />{material}</mesh>
      {[0,1,2,3].map(i => {
        const angle = (i/4)*Math.PI*2 + Math.PI/4
        return <mesh key={i} position={[Math.cos(angle)*0.2, 1.0, Math.sin(angle)*0.2]} castShadow><boxGeometry args={[0.12,0.18,0.12]} />{material}</mesh>
      })}
    </group>
  )
}

function Bishop3D({ material }: { material: ReactNode }) {
  return (
    <group>
      <mesh position={[0, 0.12, 0]} castShadow><cylinderGeometry args={[0.3, 0.35, 0.24, 32]} />{material}</mesh>
      <mesh position={[0, 0.4, 0]} castShadow><cylinderGeometry args={[0.18, 0.28, 0.32, 32]} />{material}</mesh>
      <mesh position={[0, 0.65, 0]} castShadow><cylinderGeometry args={[0.15, 0.18, 0.18, 32]} />{material}</mesh>
      <mesh position={[0, 0.78, 0]} castShadow><cylinderGeometry args={[0.18, 0.15, 0.08, 32]} />{material}</mesh>
      <mesh position={[0, 1.0, 0]} castShadow><sphereGeometry args={[0.18, 32, 32, 0, Math.PI*2, 0, Math.PI/1.5]} />{material}</mesh>
      <mesh position={[0, 1.15, 0]} castShadow><coneGeometry args={[0.08, 0.2, 16]} />{material}</mesh>
      <mesh position={[0, 1.28, 0]} castShadow><sphereGeometry args={[0.05, 16, 16]} />{material}</mesh>
    </group>
  )
}

function Knight3D({ material, isWhite }: { material: ReactNode; isWhite: boolean }) {
  return (
    <group rotation={[0, isWhite ? Math.PI : 0, 0]}>
      <mesh position={[0, 0.12, 0]} castShadow><cylinderGeometry args={[0.3, 0.35, 0.24, 32]} />{material}</mesh>
      <mesh position={[0, 0.35, 0]} castShadow><cylinderGeometry args={[0.2, 0.28, 0.22, 32]} />{material}</mesh>
      <mesh position={[0, 0.55, 0.05]} rotation={[0.3, 0, 0]} castShadow><cylinderGeometry args={[0.12, 0.18, 0.25, 16]} />{material}</mesh>
      <mesh position={[0, 0.75, 0.15]} rotation={[0.6, 0, 0]} castShadow><boxGeometry args={[0.2, 0.35, 0.3]} />{material}</mesh>
      <mesh position={[0, 0.85, 0.35]} rotation={[0.4, 0, 0]} castShadow><boxGeometry args={[0.15, 0.18, 0.25]} />{material}</mesh>
      <mesh position={[-0.08, 0.95, 0.05]} rotation={[-0.3, -0.2, 0]} castShadow><coneGeometry args={[0.05, 0.15, 8]} />{material}</mesh>
      <mesh position={[0.08, 0.95, 0.05]} rotation={[-0.3, 0.2, 0]} castShadow><coneGeometry args={[0.05, 0.15, 8]} />{material}</mesh>
      <mesh position={[0, 0.7, -0.08]} castShadow><boxGeometry args={[0.08, 0.4, 0.08]} />{material}</mesh>
    </group>
  )
}

function Pawn3D({ material }: { material: ReactNode }) {
  return (
    <group>
      <mesh position={[0, 0.1, 0]} castShadow><cylinderGeometry args={[0.25, 0.3, 0.2, 32]} />{material}</mesh>
      <mesh position={[0, 0.35, 0]} castShadow><cylinderGeometry args={[0.12, 0.22, 0.3, 32]} />{material}</mesh>
      <mesh position={[0, 0.52, 0]} castShadow><cylinderGeometry args={[0.15, 0.12, 0.06, 32]} />{material}</mesh>
      <mesh position={[0, 0.7, 0]} castShadow><sphereGeometry args={[0.15, 32, 32]} />{material}</mesh>
    </group>
  )
}

// GLTF model paths and scale factors to match basic piece sizes
// Models are in Blender meter scale (~0.05-0.095 units tall)
// Basic pieces range from ~0.85 (pawn) to ~1.55 (king) units tall
const GLTF_MODEL_PATHS: Record<PieceType, string> = {
  king: '/models/gltf/king.glb',
  queen: '/models/gltf/queen.glb',
  rook: '/models/gltf/rook.glb',
  bishop: '/models/gltf/bishop.glb',
  knight: '/models/gltf/knight.glb',
  pawn: '/models/gltf/pawn.glb',
}

const GLTF_PIECE_SCALES: Record<PieceType, number> = {
  king: 16,    // 0.095 * 16 = 1.52
  queen: 15,   // 0.085 * 15 = 1.28
  bishop: 14,  // 0.070 * 14 = 0.98
  rook: 14,    // 0.055 * 14 = 0.77
  knight: 15,  // 0.060 * 15 = 0.90
  pawn: 14,    // 0.050 * 14 = 0.70
}

// Preload all GLTF models
Object.values(GLTF_MODEL_PATHS).forEach((path) => {
  useGLTF.preload(path)
})

interface GLTFPiece3DProps {
  type: PieceType
  isWhite: boolean
  isSelected?: boolean
  isInCheck?: boolean
  pieceMaterial?: PieceMaterial
}

function GLTFPiece3D({ type, isWhite, isSelected, isInCheck, pieceMaterial = 'simple' }: GLTFPiece3DProps) {
  const { scene } = useGLTF(GLTF_MODEL_PATHS[type])

  // Clone the scene so each piece has its own instance
  const clonedScene = useMemo(() => {
    const clone = scene.clone(true)

    // Check for crystal/chrome material
    const specialMat = createSpecialMaterial(pieceMaterial, isWhite, isSelected, isInCheck)

    if (specialMat) {
      clone.traverse((child) => {
        if ((child as THREE.Mesh).isMesh) {
          const mesh = child as THREE.Mesh
          mesh.castShadow = true
          mesh.receiveShadow = true
          mesh.material = specialMat
        }
      })
      return clone
    }

    // Default material
    const color = isWhite ? '#f5f5f5' : '#3d3d3d'
    const emissive = isWhite ? '#ffffff' : '#4a4a4a'
    const selectedEmissive = isWhite ? '#88ff88' : '#44aa44'
    const checkEmissive = '#ff3333'

    let finalEmissive = emissive
    let finalEmissiveIntensity = 0.08

    if (isInCheck) {
      finalEmissive = checkEmissive
      finalEmissiveIntensity = 0.5
    } else if (isSelected) {
      finalEmissive = selectedEmissive
      finalEmissiveIntensity = 0.2
    }

    clone.traverse((child) => {
      if ((child as THREE.Mesh).isMesh) {
        const mesh = child as THREE.Mesh
        mesh.castShadow = true
        mesh.receiveShadow = true
        mesh.material = new THREE.MeshStandardMaterial({
          color: color,
          metalness: 0.3,
          roughness: 0.4,
          emissive: finalEmissive,
          emissiveIntensity: finalEmissiveIntensity,
        })
      }
    })

    return clone
  }, [scene, isWhite, isSelected, isInCheck, pieceMaterial])

  // Knight needs to face the right direction based on color
  const rotation = type === 'knight' ? (isWhite ? Math.PI : 0) : 0

  const scale = GLTF_PIECE_SCALES[type]

  return (
    <primitive
      object={clonedScene}
      scale={scale}
      rotation={[0, rotation, 0]}
    />
  )
}

// Detailed (Poly Haven) chess piece model
const DETAILED_MODEL_PATH = '/models/detailed/chess_set.glb'
useGLTF.preload(DETAILED_MODEL_PATH)

// Map piece types to node name prefixes in the Poly Haven model
const DETAILED_NODE_NAMES: Record<PieceType, string> = {
  king: 'piece_king_white',
  queen: 'piece_queen_white',
  rook: 'piece_rook_white_01',
  bishop: 'piece_bishop_white_01',
  knight: 'piece_knight_white_01',
  pawn: 'piece_pawn_white_01',
}

// The Poly Haven pieces sit on a board spanning ~±0.203 units (8 squares)
// Each square ≈ 0.0507 units. Our board has 1-unit squares.
// Scale factor: 1 / 0.0507 ≈ 19.7
const DETAILED_SCALE = 19.7

interface DetailedPiece3DFullProps extends GLTFPiece3DProps {
  pieceMaterial?: PieceMaterial
}

function DetailedPiece3D({ type, isWhite, isSelected, isInCheck, pieceMaterial = 'simple' }: DetailedPiece3DFullProps) {
  const { scene } = useGLTF(DETAILED_MODEL_PATH)

  const clonedPiece = useMemo(() => {
    // Find the matching node for this piece type
    const prefix = isWhite
      ? DETAILED_NODE_NAMES[type]
      : DETAILED_NODE_NAMES[type].replace('white', 'black')

    let sourceNode: THREE.Object3D | null = null
    scene.traverse((child) => {
      if (child.name === prefix && !sourceNode) {
        sourceNode = child
      }
    })

    if (!sourceNode) return new THREE.Group()

    const clone = (sourceNode as THREE.Object3D).clone(true)
    // Reset position and rotation since the original has board placement offsets/orientations
    clone.position.set(0, 0, 0)
    clone.rotation.set(0, 0, 0)

    // Crystal/Chrome: apply special material to all meshes and return early
    const specialMat = createSpecialMaterial(pieceMaterial, isWhite, isSelected, isInCheck)
    if (specialMat) {
      clone.traverse((child) => {
        if ((child as THREE.Mesh).isMesh) {
          const mesh = child as THREE.Mesh
          mesh.castShadow = true
          mesh.receiveShadow = true
          mesh.material = specialMat
        }
      })
      return clone
    }

    const checkEmissive = '#ff3333'
    const selectedEmissive = isWhite ? '#88ff88' : '#44aa44'

    if (pieceMaterial === 'realistic') {
      // Realistic mode: preserve original diffuse + AO textures from the Poly Haven model.
      // CRITICAL: Create fresh MeshStandardMaterial (not clone) to avoid hidden
      // MeshPhysicalMaterial properties (clearcoat, sheen, transmission, ior)
      // that cause specular aliasing (moiré).
      let emissive = isWhite ? '#ffffff' : '#4a4a4a'
      let emissiveIntensity = 0.05
      if (isInCheck) {
        emissive = checkEmissive
        emissiveIntensity = 0.5
      } else if (isSelected) {
        emissive = selectedEmissive
        emissiveIntensity = 0.2
      }

      clone.traverse((child) => {
        if ((child as THREE.Mesh).isMesh) {
          const mesh = child as THREE.Mesh
          mesh.castShadow = true
          mesh.receiveShadow = true

          const origMat = (Array.isArray(mesh.material) ? mesh.material[0] : mesh.material) as THREE.MeshStandardMaterial

          let diffuseMap: THREE.Texture | null = null
          let aoMap: THREE.Texture | null = null

          if (origMat && origMat instanceof THREE.MeshStandardMaterial) {
            if (origMat.map) {
              diffuseMap = origMat.map
              diffuseMap.anisotropy = 16
              diffuseMap.minFilter = THREE.LinearMipmapLinearFilter
              diffuseMap.magFilter = THREE.LinearFilter
              diffuseMap.generateMipmaps = true
              diffuseMap.needsUpdate = true
            }
            if (origMat.aoMap) {
              aoMap = origMat.aoMap
              aoMap.anisotropy = 16
              aoMap.needsUpdate = true
            }
          }

          const freshMat = new THREE.MeshStandardMaterial({
            color: diffuseMap ? '#ffffff' : (isWhite ? '#f0ece4' : '#2a2a2a'),
            map: diffuseMap,
            aoMap: aoMap,
            metalness: 0.02,
            roughness: 0.88,
            envMapIntensity: 0,
            emissive: emissive,
            emissiveIntensity: emissiveIntensity,
          })

          mesh.material = freshMat
        }
      })
    } else {
      // Simple mode: clean solid-color materials, no textures
      const color = isWhite ? '#f0ece4' : '#2a2a2a'
      const emissive = isWhite ? '#ffffff' : '#4a4a4a'

      let finalEmissive = emissive
      let finalEmissiveIntensity = 0.05

      if (isInCheck) {
        finalEmissive = checkEmissive
        finalEmissiveIntensity = 0.5
      } else if (isSelected) {
        finalEmissive = selectedEmissive
        finalEmissiveIntensity = 0.2
      }

      const cleanMaterial = new THREE.MeshStandardMaterial({
        color: color,
        metalness: 0.05,
        roughness: 0.45,
        emissive: finalEmissive,
        emissiveIntensity: finalEmissiveIntensity,
        envMapIntensity: 0.2,
      })

      clone.traverse((child) => {
        if ((child as THREE.Mesh).isMesh) {
          const mesh = child as THREE.Mesh
          mesh.castShadow = true
          mesh.receiveShadow = true
          mesh.material = cleanMaterial
        }
      })
    }

    return clone
  }, [scene, type, isWhite, isSelected, isInCheck, pieceMaterial])

  // Piece rotations (relative to reset rotation):
  // - Rooks: rotate -90 degrees for both colors
  // - Knights: no rotation needed — model has separate white/black geometry already facing correctly
  // - Bishops: rotate 90 degrees for both colors
  let rotation = 0
  if (type === 'rook') {
    rotation = -Math.PI / 2
  } else if (type === 'bishop') {
    rotation = Math.PI / 2
  }

  return (
    <primitive
      object={clonedPiece}
      scale={DETAILED_SCALE}
      rotation={[0, rotation, 0]}
    />
  )
}

// Fantasy (DnD) chess piece models - individual GLB files
// CC-BY 4.0 by giantcanadianpianist (Thingiverse)
const FANTASY_MODEL_PATHS: Record<PieceType, string> = {
  king: '/models/fantasy/king.glb',
  queen: '/models/fantasy/queen.glb',
  rook: '/models/fantasy/rook.glb',
  bishop: '/models/fantasy/bishop.glb',
  knight: '/models/fantasy/knight.glb',
  pawn: '/models/fantasy/pawn.glb',
}

// Scale factors: raw heights (from STL) → target board heights
// King: 58.6→1.5, Queen: 48.1→1.3, Bishop: 52.0→1.0, Knight: 44.3→0.9, Rook: 30.7→0.8, Pawn: 39.8→0.7
const FANTASY_PIECE_SCALES: Record<PieceType, number> = {
  king: 0.026,    // 58.6 * 0.026 ≈ 1.52
  queen: 0.027,   // 48.1 * 0.027 ≈ 1.30
  bishop: 0.019,  // 52.0 * 0.019 ≈ 0.99
  rook: 0.026,    // 30.7 * 0.026 ≈ 0.80
  knight: 0.020,  // 44.3 * 0.020 ≈ 0.89
  pawn: 0.018,    // 39.8 * 0.018 ≈ 0.72
}

// Preload all fantasy models
Object.values(FANTASY_MODEL_PATHS).forEach((path) => {
  useGLTF.preload(path)
})

function FantasyPiece3D({ type, isWhite, isSelected, isInCheck, pieceMaterial = 'simple' }: GLTFPiece3DProps) {
  const { scene } = useGLTF(FANTASY_MODEL_PATHS[type])

  const clonedScene = useMemo(() => {
    const clone = scene.clone(true)

    // Crystal/Chrome/Wood material override
    const specialMat = createSpecialMaterial(pieceMaterial, isWhite, isSelected, isInCheck)

    if (specialMat) {
      clone.traverse((child) => {
        if ((child as THREE.Mesh).isMesh) {
          const mesh = child as THREE.Mesh
          ensureMeshUVs(mesh.geometry) // Fantasy STL models lack UVs — generate for texture maps
          mesh.castShadow = true
          mesh.receiveShadow = true
          mesh.material = specialMat
        }
      })
      return clone
    }

    const color = isWhite ? '#f0ece4' : '#2a2a2a'
    const emissive = isWhite ? '#ffffff' : '#4a4a4a'
    const selectedEmissive = isWhite ? '#88ff88' : '#44aa44'
    const checkEmissive = '#ff3333'

    let finalEmissive = emissive
    let finalEmissiveIntensity = 0.05

    if (isInCheck) {
      finalEmissive = checkEmissive
      finalEmissiveIntensity = 0.5
    } else if (isSelected) {
      finalEmissive = selectedEmissive
      finalEmissiveIntensity = 0.2
    }

    const material = new THREE.MeshStandardMaterial({
      color: color,
      metalness: 0.08,
      roughness: 0.5,
      emissive: finalEmissive,
      emissiveIntensity: finalEmissiveIntensity,
      envMapIntensity: 0.2,
    })

    clone.traverse((child) => {
      if ((child as THREE.Mesh).isMesh) {
        const mesh = child as THREE.Mesh
        mesh.castShadow = true
        mesh.receiveShadow = true
        mesh.material = material
      }
    })

    return clone
  }, [scene, isWhite, isSelected, isInCheck, pieceMaterial])

  const scale = FANTASY_PIECE_SCALES[type]

  // Black pieces face opposite direction on the board
  const rotation = isWhite ? 0 : Math.PI

  return (
    <primitive
      object={clonedScene}
      scale={scale}
      rotation={[0, rotation, 0]}
    />
  )
}

// Meshy (AI-generated fantasy) chess piece models — individual GLB files
const MESHY_MODEL_PATHS: Record<PieceType, string> = {
  king: '/models/meshy/king.glb',
  queen: '/models/meshy/queen.glb',
  rook: '/models/meshy/rook.glb',
  bishop: '/models/meshy/bishop.glb',
  knight: '/models/meshy/knight.glb',
  pawn: '/models/meshy/pawn.glb',
}

// Scale factors: raw heights all ~1.9 units → target board heights
// Pawn must be ≤75% of smallest non-pawn (rook at ~1.05 → max pawn ~0.79)
const MESHY_PIECE_SCALES: Record<PieceType, number> = {
  king: 0.79,     // 1.9 * 0.79 ≈ 1.50
  queen: 0.68,    // 1.9 * 0.68 ≈ 1.30
  bishop: 0.63,   // 1.9 * 0.63 ≈ 1.20
  rook: 0.55,     // 1.9 * 0.55 ≈ 1.05
  knight: 0.58,   // 1.9 * 0.58 ≈ 1.10
  pawn: 0.41,     // 1.9 * 0.41 ≈ 0.78 (≤75% of rook 1.05)
}

// Preload all meshy models
Object.values(MESHY_MODEL_PATHS).forEach((path) => {
  useGLTF.preload(path)
})

function MeshyPiece3D({ type, isWhite, isSelected, isInCheck, pieceMaterial = 'simple' }: GLTFPiece3DProps) {
  const { scene } = useGLTF(MESHY_MODEL_PATHS[type])

  const clonedScene = useMemo(() => {
    const clone = scene.clone(true)

    // Crystal/Chrome/Wood material override
    const specialMat = createSpecialMaterial(pieceMaterial, isWhite, isSelected, isInCheck)

    if (specialMat) {
      clone.traverse((child) => {
        if ((child as THREE.Mesh).isMesh) {
          const mesh = child as THREE.Mesh
          ensureMeshUVs(mesh.geometry) // Meshy models lack UVs — generate for texture maps
          mesh.castShadow = true
          mesh.receiveShadow = true
          mesh.material = specialMat
        }
      })
      return clone
    }

    const checkEmissive = '#ff3333'
    const selectedEmissive = isWhite ? '#88ff88' : '#44aa44'

    if (pieceMaterial === 'realistic') {
      // Realistic mode: stone-like surfacing with visible striations and grain.
      // Meshy models have no textures or UVs, so we generate UVs and apply a
      // procedural stone texture that mimics the Traditional Detailed realistic look.
      let emissive = isWhite ? '#ffffff' : '#4a4a4a'
      let emissiveIntensity = 0.05
      if (isInCheck) {
        emissive = checkEmissive
        emissiveIntensity = 0.5
      } else if (isSelected) {
        emissive = selectedEmissive
        emissiveIntensity = 0.2
      }

      const stoneCacheKey = `stone-${isWhite}`
      let stoneMaterial = _materialCache.get(stoneCacheKey) as THREE.MeshStandardMaterial | undefined
      if (!stoneMaterial) {
        const stoneTexKey = isWhite ? 'stone-white' : 'stone-black'
        let stoneTexture = _woodTextureCache.get(stoneTexKey) // reuse texture cache map
        if (!stoneTexture) {
          stoneTexture = createStoneTexture(isWhite)
          _woodTextureCache.set(stoneTexKey, stoneTexture)
        }
        stoneMaterial = new THREE.MeshStandardMaterial({
          color: '#ffffff',
          map: stoneTexture,
          metalness: 0.02,
          roughness: 0.88,
          envMapIntensity: 0,
          emissive: emissive,
          emissiveIntensity: emissiveIntensity,
        })
        _materialCache.set(stoneCacheKey, stoneMaterial)
      }
      // Update emissive on cached material
      stoneMaterial.emissive.set(emissive)
      stoneMaterial.emissiveIntensity = emissiveIntensity

      clone.traverse((child) => {
        if ((child as THREE.Mesh).isMesh) {
          const mesh = child as THREE.Mesh
          ensureMeshUVs(mesh.geometry)
          mesh.castShadow = true
          mesh.receiveShadow = true
          mesh.material = stoneMaterial
        }
      })
      return clone
    }

    // Simple mode: clean solid-color materials
    const color = isWhite ? '#f0ece4' : '#2a2a2a'
    const emissive = isWhite ? '#ffffff' : '#4a4a4a'

    let finalEmissive = emissive
    let finalEmissiveIntensity = 0.05

    if (isInCheck) {
      finalEmissive = checkEmissive
      finalEmissiveIntensity = 0.5
    } else if (isSelected) {
      finalEmissive = selectedEmissive
      finalEmissiveIntensity = 0.2
    }

    const material = new THREE.MeshStandardMaterial({
      color: color,
      metalness: 0.08,
      roughness: 0.5,
      emissive: finalEmissive,
      emissiveIntensity: finalEmissiveIntensity,
      envMapIntensity: 0.2,
    })

    clone.traverse((child) => {
      if ((child as THREE.Mesh).isMesh) {
        const mesh = child as THREE.Mesh
        mesh.castShadow = true
        mesh.receiveShadow = true
        mesh.material = material
      }
    })

    return clone
  }, [scene, isWhite, isSelected, isInCheck, pieceMaterial])

  const scale = MESHY_PIECE_SCALES[type]

  // Black pieces face opposite direction on the board
  const rotation = isWhite ? 0 : Math.PI

  return (
    <primitive
      object={clonedScene}
      scale={scale}
      rotation={[0, rotation, 0]}
    />
  )
}

// Cubist chess pieces — procedural geometry inspired by Analytical Cubism.
// Multiple angular planes, offset blocks, and fragmented forms that remain
// recognizable as chess pieces while evoking Picasso/Braque aesthetics.

function CubistPiece3D({ type, isWhite, isSelected, isInCheck, pieceMaterial = 'simple' }: GLTFPiece3DProps) {
  const material = useMemo(() => {
    const specialMat = createSpecialMaterial(pieceMaterial, isWhite, isSelected, isInCheck)
    if (specialMat) return specialMat

    const checkEmissive = '#ff3333'
    const selectedEmissive = isWhite ? '#88ff88' : '#44aa44'
    let emissive = isWhite ? '#ffffff' : '#4a4a4a'
    let emissiveIntensity = 0.05
    if (isInCheck) { emissive = checkEmissive; emissiveIntensity = 0.5 }
    else if (isSelected) { emissive = selectedEmissive; emissiveIntensity = 0.2 }

    return new THREE.MeshStandardMaterial({
      color: isWhite ? '#f0ece4' : '#2a2a2a',
      metalness: 0.1,
      roughness: 0.6,
      flatShading: true, // Enhances the faceted cubist aesthetic
      emissive: new THREE.Color(emissive),
      emissiveIntensity,
      envMapIntensity: 0.15,
    })
  }, [pieceMaterial, isWhite, isSelected, isInCheck])

  const rotation = isWhite ? 0 : Math.PI

  const M = ({ p, r, s }: { p: [number, number, number]; r?: [number, number, number]; s: [number, number, number] }) => (
    <mesh position={p} rotation={r || [0, 0, 0]} material={material} castShadow receiveShadow>
      <boxGeometry args={s} />
    </mesh>
  )

  let pieceGeometry: React.ReactNode = null

  if (type === 'king') {
    // Tallest piece: angular tower with offset planes and fragmented cross
    pieceGeometry = (
      <>
        <M p={[0, 0.03, 0]} s={[0.5, 0.06, 0.5]} />
        <M p={[0.02, 0.12, -0.01]} r={[0, 0.08, 0]} s={[0.4, 0.12, 0.42]} />
        <M p={[-0.01, 0.26, 0.02]} r={[0, -0.06, 0.03]} s={[0.34, 0.16, 0.32]} />
        <M p={[0.02, 0.42, -0.01]} r={[0, 0.05, -0.02]} s={[0.38, 0.04, 0.24]} />
        <M p={[-0.01, 0.62, 0.01]} r={[0, -0.1, 0.02]} s={[0.26, 0.32, 0.24]} />
        <M p={[0.03, 0.84, -0.02]} r={[0, 0.12, 0]} s={[0.34, 0.05, 0.2]} />
        <M p={[-0.02, 1.0, 0.01]} r={[0, -0.15, 0.04]} s={[0.22, 0.22, 0.2]} />
        <M p={[0, 1.2, 0]} r={[0, 0.08, 0.06]} s={[0.06, 0.28, 0.06]} />
        <M p={[0.01, 1.22, 0]} r={[0, -0.05, 0.04]} s={[0.24, 0.06, 0.06]} />
        <M p={[-0.06, 1.32, 0.02]} r={[0, 0, -0.15]} s={[0.04, 0.12, 0.04]} />
      </>
    )
  } else if (type === 'queen') {
    // Elegant angular spire — tilted planes ascending to a pointed crown
    pieceGeometry = (
      <>
        <M p={[0, 0.03, 0]} s={[0.48, 0.06, 0.48]} />
        <M p={[0.01, 0.12, -0.01]} r={[0, 0.06, 0]} s={[0.38, 0.12, 0.4]} />
        <M p={[-0.02, 0.26, 0.02]} r={[0, -0.08, 0.02]} s={[0.32, 0.16, 0.3]} />
        <M p={[0.01, 0.42, -0.01]} r={[0, 0.1, -0.03]} s={[0.28, 0.16, 0.26]} />
        <M p={[-0.02, 0.58, 0.01]} r={[0, -0.05, 0.02]} s={[0.24, 0.16, 0.22]} />
        <M p={[0.02, 0.74, -0.01]} r={[0, 0.12, -0.02]} s={[0.2, 0.16, 0.18]} />
        <M p={[-0.01, 0.88, 0]} r={[0, -0.08, 0]} s={[0.3, 0.04, 0.16]} />
        {/* Crown: rotated diamond */}
        <M p={[0, 1.0, 0]} r={[0, Math.PI / 4, Math.PI / 4]} s={[0.14, 0.14, 0.14]} />
        <M p={[0.03, 1.12, -0.02]} r={[0, -0.2, 0.1]} s={[0.08, 0.08, 0.08]} />
        <mesh position={[0, 1.2, 0]} material={material} castShadow>
          <sphereGeometry args={[0.05, 8, 8]} />
        </mesh>
      </>
    )
  } else if (type === 'bishop') {
    // Leaning form suggesting diagonal movement — angular mitre on top
    pieceGeometry = (
      <>
        <M p={[0, 0.03, 0]} s={[0.42, 0.06, 0.42]} />
        <M p={[0.01, 0.12, 0]} r={[0, 0.05, 0]} s={[0.34, 0.12, 0.36]} />
        <M p={[-0.02, 0.26, 0.02]} r={[0, -0.04, 0.06]} s={[0.28, 0.16, 0.26]} />
        {/* Leaning body */}
        <M p={[0.04, 0.46, -0.01]} r={[0, 0.08, 0.1]} s={[0.22, 0.24, 0.2]} />
        <M p={[0.08, 0.68, -0.02]} r={[0, -0.06, 0.08]} s={[0.18, 0.2, 0.16]} />
        {/* Diagonal slash */}
        <M p={[0.02, 0.55, 0.01]} r={[0, 0.3, 0.7]} s={[0.28, 0.02, 0.12]} />
        {/* Angular mitre */}
        <M p={[0.1, 0.84, -0.02]} r={[0, 0.15, 0.12]} s={[0.14, 0.14, 0.1]} />
        <M p={[0.12, 0.96, -0.01]} r={[0, -0.1, 0.08]} s={[0.08, 0.12, 0.08]} />
        <mesh position={[0.13, 1.06, -0.01]} material={material} castShadow>
          <sphereGeometry args={[0.04, 8, 8]} />
        </mesh>
      </>
    )
  } else if (type === 'knight') {
    // Abstract horse head — intersecting angular planes
    pieceGeometry = (
      <group rotation={[0, rotation, 0]}>
        <M p={[0, 0.03, 0]} s={[0.44, 0.06, 0.42]} />
        <M p={[0.01, 0.12, 0]} r={[0, 0.05, 0]} s={[0.34, 0.12, 0.34]} />
        {/* Neck — angled forward */}
        <M p={[0, 0.3, 0.04]} r={[0.15, 0.06, 0]} s={[0.22, 0.24, 0.2]} />
        {/* Head main plane — tall, thin, angled */}
        <M p={[0, 0.54, 0.12]} r={[0.25, 0, 0.05]} s={[0.2, 0.26, 0.08]} />
        {/* Head side plane — intersecting */}
        <M p={[0, 0.5, 0.14]} r={[0.2, 0.3, 0]} s={[0.08, 0.22, 0.18]} />
        {/* Muzzle — protruding forward */}
        <M p={[0, 0.52, 0.28]} r={[0.4, 0, 0.08]} s={[0.14, 0.1, 0.16]} />
        {/* Jaw */}
        <M p={[0, 0.42, 0.24]} r={[0.5, 0, 0]} s={[0.12, 0.04, 0.14]} />
        {/* Ear 1 */}
        <M p={[-0.06, 0.72, 0.06]} r={[-0.2, -0.15, -0.2]} s={[0.04, 0.14, 0.06]} />
        {/* Ear 2 */}
        <M p={[0.06, 0.7, 0.08]} r={[-0.15, 0.2, 0.15]} s={[0.04, 0.12, 0.05]} />
        {/* Mane accent */}
        <M p={[0, 0.6, -0.02]} r={[0.1, 0.1, 0.3]} s={[0.16, 0.08, 0.04]} />
      </group>
    )
    // Knight handles its own rotation via the inner group, so skip the outer rotation
    return (
      <group>
        {pieceGeometry}
      </group>
    )
  } else if (type === 'rook') {
    // Tower of stacked offset cubes with angular battlements
    pieceGeometry = (
      <>
        <M p={[0, 0.03, 0]} s={[0.46, 0.06, 0.46]} />
        <M p={[0.02, 0.14, -0.01]} r={[0, 0.05, 0]} s={[0.4, 0.16, 0.4]} />
        <M p={[-0.02, 0.3, 0.01]} r={[0, -0.08, 0]} s={[0.34, 0.16, 0.34]} />
        <M p={[0.01, 0.44, -0.02]} r={[0, 0.06, 0]} s={[0.3, 0.12, 0.3]} />
        <M p={[-0.01, 0.56, 0.01]} r={[0, -0.04, 0]} s={[0.36, 0.12, 0.36]} />
        {/* Battlements: 4 offset boxes on corners */}
        <M p={[0.14, 0.72, 0.14]} r={[0, 0.1, 0]} s={[0.1, 0.14, 0.1]} />
        <M p={[-0.14, 0.74, 0.14]} r={[0, -0.12, 0]} s={[0.1, 0.16, 0.1]} />
        <M p={[0.14, 0.73, -0.14]} r={[0, 0.08, 0]} s={[0.1, 0.12, 0.1]} />
        <M p={[-0.14, 0.71, -0.14]} r={[0, -0.06, 0]} s={[0.1, 0.14, 0.1]} />
      </>
    )
  } else {
    // Pawn: simple geometric — octahedral body with offset head
    pieceGeometry = (
      <>
        <M p={[0, 0.03, 0]} s={[0.38, 0.06, 0.38]} />
        <M p={[0.01, 0.12, -0.01]} r={[0, 0.06, 0]} s={[0.3, 0.12, 0.3]} />
        {/* Octahedral body: rotated cube */}
        <M p={[0, 0.3, 0]} r={[0, Math.PI / 4, 0]} s={[0.2, 0.2, 0.2]} />
        <M p={[-0.02, 0.3, 0.01]} r={[Math.PI / 6, 0, Math.PI / 6]} s={[0.18, 0.18, 0.18]} />
        {/* Neck */}
        <M p={[0.01, 0.44, 0]} r={[0, -0.05, 0.03]} s={[0.12, 0.08, 0.12]} />
        {/* Head */}
        <mesh position={[-0.01, 0.54, 0.01]} material={material} castShadow>
          <sphereGeometry args={[0.1, 8, 8]} />
        </mesh>
        {/* Accent plane through head */}
        <M p={[0, 0.54, 0]} r={[0, 0.4, 0.2]} s={[0.18, 0.02, 0.14]} />
      </>
    )
  }

  return (
    <group rotation={[0, rotation, 0]}>
      {pieceGeometry}
    </group>
  )
}

// SVG piece sprite cache: load each SVG once, render to canvas texture
const svgTextureCache = new Map<string, HTMLImageElement>()
const svgLoadPromises = new Map<string, Promise<HTMLImageElement>>()

function loadPieceSVG(key: string): Promise<HTMLImageElement> {
  if (svgLoadPromises.has(key)) return svgLoadPromises.get(key)!
  const promise = new Promise<HTMLImageElement>((resolve, reject) => {
    const img = new Image()
    img.onload = () => { svgTextureCache.set(key, img); resolve(img) }
    img.onerror = reject
    const pieceCode = { king: 'K', queen: 'Q', rook: 'R', bishop: 'B', knight: 'N', pawn: 'P' }
    const color = key.startsWith('w') ? 'w' : 'b'
    const piece = pieceCode[key.slice(1) as keyof typeof pieceCode]
    img.src = `/pieces/${color}${piece}.svg`
  })
  svgLoadPromises.set(key, promise)
  return promise
}

function Piece2D({ type, isWhite, isSelected, isInCheck, boardYRotation = 0 }: PieceRenderProps) {
  const [svgLoaded, setSvgLoaded] = useState(false)
  const svgKey = `${isWhite ? 'w' : 'b'}${type}`

  useEffect(() => {
    if (svgTextureCache.has(svgKey)) { setSvgLoaded(true); return }
    loadPieceSVG(svgKey).then(() => setSvgLoaded(true))
  }, [svgKey])

  const texture = useMemo(() => {
    const canvas = document.createElement('canvas')
    const size = 256
    canvas.width = size
    canvas.height = size
    const ctx = canvas.getContext('2d')!
    ctx.clearRect(0, 0, size, size)

    const img = svgTextureCache.get(svgKey)
    if (img) {
      // Draw SVG centered with padding
      const padding = 20
      const drawSize = size - padding * 2
      ctx.drawImage(img, padding, padding, drawSize, drawSize)
    }

    // Draw check indicator (red ring)
    if (isInCheck) {
      ctx.strokeStyle = '#ff3333'
      ctx.lineWidth = 10
      ctx.shadowColor = '#ff3333'
      ctx.shadowBlur = 15
      ctx.beginPath()
      ctx.arc(size / 2, size / 2, size / 2 - 12, 0, Math.PI * 2)
      ctx.stroke()
      ctx.shadowBlur = 0
    }

    // Draw selection indicator (green ring)
    if (isSelected) {
      ctx.strokeStyle = '#4CAF50'
      ctx.lineWidth = 8
      ctx.beginPath()
      ctx.arc(size / 2, size / 2, size / 2 - 8, 0, Math.PI * 2)
      ctx.stroke()
    }

    const tex = new THREE.CanvasTexture(canvas)
    tex.needsUpdate = true
    return tex
  }, [type, isWhite, isSelected, isInCheck, svgLoaded])

  return (
    <group rotation={[-Math.PI / 2 + boardYRotation, -boardYRotation, 0]}>
      <mesh position={[0, 0, 0.01]} renderOrder={10}>
        <planeGeometry args={[0.9, 0.9]} />
        <meshBasicMaterial map={texture} transparent alphaTest={0.1} side={THREE.DoubleSide} depthWrite={false} />
      </mesh>
    </group>
  )
}

// Chess piece SVGs: Cburnett set from Wikimedia Commons (CC-BY-SA 3.0)
