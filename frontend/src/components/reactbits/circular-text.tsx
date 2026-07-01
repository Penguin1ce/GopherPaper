"use client";

import { useEffect } from "react";
import {
  motion,
  type MotionValue,
  type Transition,
  useAnimation,
  useMotionValue,
} from "motion/react";

import { cn } from "@/lib/utils";

type CircularTextProps = {
  text: string;
  spinDuration?: number;
  onHover?: "slowDown" | "speedUp" | "pause" | "goBonkers";
  className?: string;
  letterClassName?: string;
};

const getRotationTransition = (duration: number, from: number, loop = true) => ({
  from,
  to: from + 360,
  ease: "linear" as const,
  duration,
  type: "tween" as const,
  repeat: loop ? Infinity : 0,
});

const getTransition = (duration: number, from: number) => ({
  rotate: getRotationTransition(duration, from),
  scale: {
    type: "spring" as const,
    damping: 20,
    stiffness: 300,
  },
});

export function CircularText({
  text,
  spinDuration = 20,
  onHover = "speedUp",
  className,
  letterClassName,
}: CircularTextProps) {
  const letters = Array.from(text);
  const controls = useAnimation();
  const rotation: MotionValue<number> = useMotionValue(0);

  useEffect(() => {
    const start = rotation.get();
    controls.start({
      rotate: start + 360,
      scale: 1,
      transition: getTransition(spinDuration, start),
    });
  }, [controls, onHover, rotation, spinDuration, text]);

  const handleHoverStart = () => {
    const start = rotation.get();
    let transitionConfig: ReturnType<typeof getTransition> | Transition;
    let scaleVal = 1;

    switch (onHover) {
      case "slowDown":
        transitionConfig = getTransition(spinDuration * 2, start);
        break;
      case "speedUp":
        transitionConfig = getTransition(spinDuration / 4, start);
        break;
      case "pause":
        transitionConfig = {
          rotate: { type: "spring", damping: 20, stiffness: 300 },
          scale: { type: "spring", damping: 20, stiffness: 300 },
        };
        break;
      case "goBonkers":
        transitionConfig = getTransition(spinDuration / 20, start);
        scaleVal = 0.8;
        break;
      default:
        transitionConfig = getTransition(spinDuration, start);
    }

    controls.start({
      rotate: start + 360,
      scale: scaleVal,
      transition: transitionConfig,
    });
  };

  const handleHoverEnd = () => {
    const start = rotation.get();
    controls.start({
      rotate: start + 360,
      scale: 1,
      transition: getTransition(spinDuration, start),
    });
  };

  return (
    <motion.div
      className={cn(
        "relative m-0 rounded-full text-center font-black text-primary/70 origin-center",
        className,
      )}
      style={{ rotate: rotation }}
      initial={{ rotate: 0 }}
      animate={controls}
      onMouseEnter={handleHoverStart}
      onMouseLeave={handleHoverEnd}
    >
      {letters.map((letter, index) => {
        const rotationDeg = (360 / letters.length) * index;
        const factor = Math.PI / letters.length;
        const offset = factor * index;
        const transform = `rotateZ(${rotationDeg}deg) translate3d(${offset}px, ${offset}px, 0)`;

        return (
          <span
            key={`${letter}-${index}`}
            className={cn(
              "absolute inset-0 inline-block text-[0.7rem] font-semibold leading-none tracking-[0.14em] transition-all duration-500 ease-[cubic-bezier(0,0,0,1)]",
              letterClassName,
            )}
            style={{ transform, WebkitTransform: transform }}
          >
            {letter}
          </span>
        );
      })}
    </motion.div>
  );
}
