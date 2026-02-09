import type { ReactNode } from 'react'
import { useRef, useMemo, useState } from 'react'
import { useFrame } from '@react-three/fiber'
import type { ThreeEvent } from '@react-three/fiber'
import * as THREE from 'three'
import { boardToWorld, worldToBoard } from '../hooks/useGameState'
import type { PieceState, useGameState } from '../hooks/useGameState'
import type { OnlineContext } from './ChessScene'
import { canSelectPiece, isValidMove, toAlgebraic } from '../utils/chessRules'
import type { Board } from '../utils/chessRules'

interface ChessPiecesProps {
  morphProgress: React.MutableRefObject<{ value: number }>
  gameState: ReturnType<typeof useGameState>
  is3D: boolean
  onlineContext?: OnlineContext
}

type PieceType = 'king' | 'queen' | 'rook' | 'bishop' | 'knight' | 'pawn'

// Convert gameState pieces to chess rules board format
function createBoard(pieces: PieceState[], whiteToMove: boolean): Board {
  return {
    pieces: pieces.map(p => ({
      type: p.type,
      isWhite: p.isWhite,
      file: p.file,
      rank: p.rank,
    })),
    whiteToMove,
    castlingRights: {
      whiteKingside: true,
      whiteQueenside: true,
      blackKingside: true,
      blackQueenside: true,
    },
    enPassantSquare: null,
  }
}

export function ChessPieces({ morphProgress, gameState, is3D, onlineContext }: ChessPiecesProps) {
  const { pieces, selectedPieceId, hoverSquare } = gameState

  return (
    <group>
      {pieces.map((piece) => (
        <ChessPiece
          key={piece.id}
          piece={piece}
          morphProgress={morphProgress}
          gameState={gameState}
          is3D={is3D}
          isSelected={selectedPieceId === piece.id}
          onlineContext={onlineContext}
        />
      ))}

      {/* Drop shadow indicator for selected piece in 3D mode */}
      {is3D && selectedPieceId && hoverSquare && (
        <DropShadow file={hoverSquare.file} rank={hoverSquare.rank} />
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
  onlineContext?: OnlineContext
}

function ChessPiece({ piece, morphProgress, gameState, is3D, isSelected, onlineContext }: ChessPieceProps) {
  const groupRef = useRef<THREE.Group>(null)
  const piece3DRef = useRef<THREE.Group>(null)
  const piece2DRef = useRef<THREE.Group>(null)

  // Animation state for lifted piece
  const liftProgress = useRef(0)
  const targetLift = isSelected ? 1 : 0

  // Calculate base position from board coordinates
  const basePosition = useMemo(() => boardToWorld(piece.file, piece.rank), [piece.file, piece.rank])

  // For 2D drag
  const [dragOffset, setDragOffset] = useState<[number, number]>([0, 0])
  const [isDragging, setIsDragging] = useState(false)

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

      // In 3D mode, lift the piece; in 2D mode, use drag offset
      if (is3D) {
        groupRef.current.position.set(
          basePosition[0],
          basePosition[1] + liftHeight + bobAmount,
          basePosition[2]
        )
      } else {
        groupRef.current.position.set(
          basePosition[0] + dragOffset[0],
          basePosition[1],
          basePosition[2] + dragOffset[1]
        )
      }

      // 3D piece visibility
      piece3DRef.current.visible = progress > 0.01
      piece3DRef.current.scale.setScalar(progress * (isSelected && is3D ? 1.1 : 1))
      piece3DRef.current.position.y = 0

      // 2D piece visibility
      const twoDScale = 1 - progress
      piece2DRef.current.visible = twoDScale > 0.01
      const scale2D = twoDScale * (isSelected && !is3D ? 1.15 : 1)
      piece2DRef.current.scale.set(scale2D, scale2D, 1)
      piece2DRef.current.position.y = 0.1 + (isDragging ? 0.1 : 0)
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

    // If game is waiting for opponent or complete, don't allow moves
    if (gameStatus === 'waiting' || gameStatus === 'complete') {
      return
    }

    if (isSelected) {
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
      const whiteToMove = currentTurn ? currentTurn === 'white' : selectedPiece.isWhite
      const board = createBoard(gameState.pieces, whiteToMove)
      const from = { file: selectedPiece.file, rank: selectedPiece.rank }
      const to = { file: piece.file, rank: piece.rank }
      const validation = isValidMove(board, from, to)

      if (!validation.valid) {
        console.log('Invalid move:', validation.reason, 'pieceType:', validation.pieceType)
        console.log('onInvalidMove callback exists:', !!onlineContext?.onInvalidMove)
        // Show invalid move modal
        if (onlineContext?.onInvalidMove && validation.reason && validation.pieceType) {
          console.log('Calling onInvalidMove with:', { reason: validation.reason, pieceType: validation.pieceType })
          onlineContext.onInvalidMove({
            reason: validation.reason,
            pieceType: validation.pieceType
          })
        } else {
          console.log('Not showing modal - missing:', {
            hasCallback: !!onlineContext?.onInvalidMove,
            hasReason: !!validation.reason,
            hasPieceType: !!validation.pieceType
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

  const handlePointerDown = (e: ThreeEvent<PointerEvent>) => {
    if (!is3D && !gameState.selectedPieceId) {
      const playerColor = onlineContext?.playerColor
      const currentTurn = onlineContext?.currentTurn
      const gameStatus = onlineContext?.gameStatus
      const isOnlineGame = onlineContext?.sessionId != null

      // If game is waiting or complete, don't allow interaction
      if (isOnlineGame && (gameStatus === 'waiting' || gameStatus === 'complete')) {
        return
      }

      // Check if we can select this piece
      if (!canSelectPiece(piece, playerColor || null, currentTurn || null)) {
        return
      }

      e.stopPropagation()
      setIsDragging(true)
      gameState.selectPiece(piece.id)
    }
  }

  const handlePointerUp = async (e: ThreeEvent<PointerEvent>) => {
    if (!is3D && isDragging) {
      e.stopPropagation()
      setIsDragging(false)
      const dropX = basePosition[0] + dragOffset[0]
      const dropZ = basePosition[2] + dragOffset[1]
      const targetSquare = worldToBoard(dropX, dropZ)

      if (targetSquare) {
        const isOnlineGame = onlineContext?.sessionId != null
        const currentTurn = onlineContext?.currentTurn

        // Validate the move
        const whiteToMove = currentTurn ? currentTurn === 'white' : piece.isWhite
        const board = createBoard(gameState.pieces, whiteToMove)
        const from = { file: piece.file, rank: piece.rank }
        const to = { file: targetSquare.file, rank: targetSquare.rank }
        const validation = isValidMove(board, from, to)

        if (validation.valid) {
          // If online game, send move to server
          if (isOnlineGame && onlineContext?.makeMove) {
            const fromNotation = toAlgebraic(from.file, from.rank)
            const toNotation = toAlgebraic(to.file, to.rank)
            try {
              await onlineContext.makeMove(fromNotation, toNotation)
            } catch (err) {
              console.error('Failed to make move:', err)
            }
          }

          gameState.movePiece(piece.id, targetSquare.file, targetSquare.rank)
        } else {
          console.log('Invalid move:', validation.reason)
          gameState.selectPiece(null)
        }
      } else {
        gameState.selectPiece(null)
      }
      setDragOffset([0, 0])
    }
  }

  const handlePointerMove = (e: ThreeEvent<PointerEvent>) => {
    if (!is3D && isDragging && groupRef.current) {
      e.stopPropagation()
      const movementScale = 0.02
      setDragOffset(prev => [
        prev[0] + e.movementX * movementScale,
        prev[1] + e.movementY * movementScale
      ])
    }
  }

  return (
    <group
      ref={groupRef}
      position={basePosition}
      onClick={handleClick}
      onPointerDown={handlePointerDown}
      onPointerUp={handlePointerUp}
      onPointerMove={handlePointerMove}
    >
      <group ref={piece3DRef}>
        <Piece3D type={piece.type as PieceType} isWhite={piece.isWhite} isSelected={isSelected && is3D} />
      </group>
      <group ref={piece2DRef}>
        <Piece2D type={piece.type as PieceType} isWhite={piece.isWhite} isSelected={isSelected && !is3D} />
      </group>
      {isSelected && is3D && <SelectionRing />}
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

interface PieceRenderProps {
  type: PieceType
  isWhite: boolean
  isSelected?: boolean
}

function Piece3D({ type, isWhite, isSelected }: PieceRenderProps) {
  const color = isWhite ? '#f5f5f5' : '#3d3d3d'
  const emissive = isWhite ? '#ffffff' : '#4a4a4a'
  const selectedEmissive = isWhite ? '#88ff88' : '#44aa44'

  const material = (
    <meshStandardMaterial
      color={color}
      metalness={0.3}
      roughness={0.4}
      emissive={isSelected ? selectedEmissive : emissive}
      emissiveIntensity={isSelected ? 0.2 : 0.08}
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

function Piece2D({ type, isWhite, isSelected }: PieceRenderProps) {
  const texture = useMemo(() => {
    const canvas = document.createElement('canvas')
    canvas.width = 128
    canvas.height = 128
    const ctx = canvas.getContext('2d')!
    ctx.clearRect(0, 0, 128, 128)

    const fillColor = isWhite ? '#ffffff' : '#3d3d3d'
    const strokeColor = isWhite ? '#333333' : '#d5d5d5'
    ctx.fillStyle = fillColor
    ctx.strokeStyle = strokeColor
    ctx.lineWidth = 3
    ctx.save()
    ctx.translate(64, 64)

    switch (type) {
      case 'king': drawKing(ctx, fillColor, strokeColor); break
      case 'queen': drawQueen(ctx, fillColor, strokeColor); break
      case 'rook': drawRook(ctx, fillColor, strokeColor); break
      case 'bishop': drawBishop(ctx, fillColor, strokeColor); break
      case 'knight': drawKnight(ctx, fillColor, strokeColor, isWhite); break
      case 'pawn': drawPawn(ctx, fillColor, strokeColor); break
    }
    ctx.restore()

    if (isSelected) {
      ctx.strokeStyle = '#4CAF50'
      ctx.lineWidth = 6
      ctx.beginPath()
      ctx.arc(64, 64, 58, 0, Math.PI * 2)
      ctx.stroke()
    }

    const tex = new THREE.CanvasTexture(canvas)
    tex.needsUpdate = true
    return tex
  }, [type, isWhite, isSelected])

  return (
    <group rotation={[-Math.PI / 2, 0, 0]}>
      <mesh position={[0, 0, 0.01]}>
        <planeGeometry args={[0.9, 0.9]} />
        <meshBasicMaterial map={texture} transparent alphaTest={0.1} side={THREE.DoubleSide} />
      </mesh>
    </group>
  )
}

function drawKing(ctx: CanvasRenderingContext2D, _fill: string, stroke: string) {
  ctx.beginPath()
  ctx.moveTo(-35, 45); ctx.lineTo(35, 45); ctx.lineTo(30, 35); ctx.lineTo(-30, 35); ctx.closePath()
  ctx.fill(); ctx.stroke()
  ctx.beginPath()
  ctx.moveTo(-25, 35); ctx.quadraticCurveTo(-30, 10, -20, -10); ctx.quadraticCurveTo(-15, -25, 0, -30)
  ctx.quadraticCurveTo(15, -25, 20, -10); ctx.quadraticCurveTo(30, 10, 25, 35); ctx.closePath()
  ctx.fill(); ctx.stroke()
  ctx.fillStyle = stroke
  ctx.fillRect(-4, -55, 8, 30); ctx.fillRect(-12, -45, 24, 8)
}

function drawQueen(ctx: CanvasRenderingContext2D, fill: string, stroke: string) {
  ctx.beginPath()
  ctx.moveTo(-35, 45); ctx.lineTo(35, 45); ctx.lineTo(30, 35); ctx.lineTo(-30, 35); ctx.closePath()
  ctx.fill(); ctx.stroke()
  ctx.beginPath()
  ctx.moveTo(-25, 35); ctx.quadraticCurveTo(-28, 10, -18, -10); ctx.lineTo(-25, -35); ctx.lineTo(-12, -20)
  ctx.lineTo(0, -45); ctx.lineTo(12, -20); ctx.lineTo(25, -35); ctx.lineTo(18, -10)
  ctx.quadraticCurveTo(28, 10, 25, 35); ctx.closePath()
  ctx.fill(); ctx.stroke()
  ctx.fillStyle = fill; ctx.strokeStyle = stroke
  const points: [number, number][] = [[-25, -35], [-12, -20], [0, -45], [12, -20], [25, -35]]
  points.forEach(([x, y]) => { ctx.beginPath(); ctx.arc(x, y, 6, 0, Math.PI * 2); ctx.fill(); ctx.stroke() })
}

function drawRook(ctx: CanvasRenderingContext2D, _fill: string, _stroke: string) {
  ctx.beginPath()
  ctx.moveTo(-35, 45); ctx.lineTo(35, 45); ctx.lineTo(30, 35); ctx.lineTo(-30, 35); ctx.closePath()
  ctx.fill(); ctx.stroke()
  ctx.beginPath()
  ctx.moveTo(-25, 35); ctx.lineTo(-25, -15); ctx.lineTo(-30, -15); ctx.lineTo(-30, -35)
  ctx.lineTo(-18, -35); ctx.lineTo(-18, -25); ctx.lineTo(-6, -25); ctx.lineTo(-6, -35)
  ctx.lineTo(6, -35); ctx.lineTo(6, -25); ctx.lineTo(18, -25); ctx.lineTo(18, -35)
  ctx.lineTo(30, -35); ctx.lineTo(30, -15); ctx.lineTo(25, -15); ctx.lineTo(25, 35); ctx.closePath()
  ctx.fill(); ctx.stroke()
}

function drawBishop(ctx: CanvasRenderingContext2D, fill: string, stroke: string) {
  ctx.beginPath()
  ctx.moveTo(-30, 45); ctx.lineTo(30, 45); ctx.lineTo(25, 35); ctx.lineTo(-25, 35); ctx.closePath()
  ctx.fill(); ctx.stroke()
  ctx.beginPath()
  ctx.moveTo(-20, 35); ctx.quadraticCurveTo(-25, 15, -18, 0); ctx.quadraticCurveTo(-20, -20, 0, -40)
  ctx.quadraticCurveTo(20, -20, 18, 0); ctx.quadraticCurveTo(25, 15, 20, 35); ctx.closePath()
  ctx.fill(); ctx.stroke()
  ctx.strokeStyle = stroke; ctx.lineWidth = 4
  ctx.beginPath(); ctx.moveTo(8, -30); ctx.lineTo(-8, -5); ctx.stroke(); ctx.lineWidth = 3
  ctx.fillStyle = fill; ctx.beginPath(); ctx.arc(0, -45, 7, 0, Math.PI * 2); ctx.fill(); ctx.stroke()
}

function drawKnight(ctx: CanvasRenderingContext2D, _fill: string, stroke: string, isWhite: boolean) {
  ctx.save()
  if (!isWhite) ctx.scale(-1, 1)
  ctx.beginPath()
  ctx.moveTo(-30, 45); ctx.lineTo(30, 45); ctx.lineTo(25, 35); ctx.lineTo(-25, 35); ctx.closePath()
  ctx.fill(); ctx.stroke()
  ctx.beginPath()
  ctx.moveTo(-20, 35); ctx.lineTo(-20, 10); ctx.quadraticCurveTo(-25, -5, -15, -20)
  ctx.lineTo(-20, -30); ctx.quadraticCurveTo(-15, -40, 0, -45); ctx.quadraticCurveTo(20, -45, 25, -30)
  ctx.lineTo(30, -20); ctx.lineTo(15, -15); ctx.lineTo(25, -5); ctx.quadraticCurveTo(30, 15, 20, 35); ctx.closePath()
  ctx.fill(); ctx.stroke()
  ctx.fillStyle = stroke
  ctx.beginPath(); ctx.arc(5, -25, 4, 0, Math.PI * 2); ctx.fill()
  ctx.beginPath(); ctx.arc(22, -12, 3, 0, Math.PI * 2); ctx.fill()
  ctx.restore()
}

function drawPawn(ctx: CanvasRenderingContext2D, _fill: string, _stroke: string) {
  ctx.beginPath()
  ctx.moveTo(-30, 45); ctx.lineTo(30, 45); ctx.lineTo(25, 35); ctx.lineTo(-25, 35); ctx.closePath()
  ctx.fill(); ctx.stroke()
  ctx.beginPath()
  ctx.moveTo(-18, 35); ctx.quadraticCurveTo(-22, 20, -15, 10); ctx.lineTo(-18, 5)
  ctx.quadraticCurveTo(-20, -5, -12, -10); ctx.quadraticCurveTo(-18, -20, 0, -35)
  ctx.quadraticCurveTo(18, -20, 12, -10); ctx.quadraticCurveTo(20, -5, 18, 5)
  ctx.lineTo(15, 10); ctx.quadraticCurveTo(22, 20, 18, 35); ctx.closePath()
  ctx.fill(); ctx.stroke()
}
