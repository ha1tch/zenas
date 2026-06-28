# Generate a representative instruction for (nearly) every Z80 opcode family.
regs8 = ["A","B","C","D","E","H","L"]
regs16 = ["BC","DE","HL","SP"]
conds = ["NZ","Z","NC","C","PO","PE","P","M"]
jrconds = ["NZ","Z","NC","C"]
out = []
# 8-bit loads
for d in regs8:
    for s in regs8: out.append(f"LD {d},{s}")
    out.append(f"LD {d},5")
    out.append(f"LD {d},(HL)")
    out.append(f"LD (HL),{d}")
out.append("LD (HL),5")
out += ["LD A,(BC)","LD A,(DE)","LD (BC),A","LD (DE),A","LD A,($1234)","LD ($1234),A"]
# 16-bit loads
for r in regs16: out += [f"LD {r},$1234", f"PUSH {r if r!='SP' else 'AF'}", f"POP {r if r!='SP' else 'AF'}"]
out += ["LD HL,($1234)","LD ($1234),HL","LD SP,HL","PUSH AF","POP AF"]
# ALU (reg, imm, (HL))
for op in ["ADD A,","ADC A,","SUB ","SBC A,","AND ","XOR ","OR ","CP "]:
    for s in regs8: out.append(f"{op}{s}")
    out.append(f"{op}5")
    out.append(f"{op}(HL)")
# INC/DEC
for r in regs8: out += [f"INC {r}", f"DEC {r}"]
out += ["INC (HL)","DEC (HL)"]
for r in regs16: out += [f"INC {r}", f"DEC {r}"]
# 16-bit arith
for r in regs16: out.append(f"ADD HL,{r}")
for r in regs16: out += [f"ADC HL,{r}", f"SBC HL,{r}"]
# rotates/shifts on reg and (HL)
for op in ["RLC","RRC","RL","RR","SLA","SRA","SRL"]:
    for s in regs8: out.append(f"{op} {s}")
    out.append(f"{op} (HL)")
out += ["RLCA","RRCA","RLA","RRA","RLD","RRD","DAA","CPL","NEG","CCF","SCF"]
# bit ops
for b in range(8):
    for op in ["BIT","RES","SET"]:
        out.append(f"{op} {b},A"); out.append(f"{op} {b},(HL)")
# jumps/calls/ret
out += ["JP $1234","JP (HL)","JR $8000","DJNZ $8000","CALL $1234","RET","RETI","RETN","NOP","HALT","DI","EI"]
for c in conds: out += [f"JP {c},$1234", f"CALL {c},$1234", f"RET {c}"]
for c in jrconds: out.append(f"JR {c},$8000")
for n in [0,8,0x10,0x18,0x20,0x28,0x30,0x38]: out.append(f"RST ${n:02X}")
out += ["IM 0","IM 1","IM 2","EX DE,HL","EX AF,AF'","EXX","EX (SP),HL"]
# block ops
out += ["LDI","LDD","LDIR","LDDR","CPI","CPD","CPIR","CPDR","INI","IND","INIR","INDR","OUTI","OUTD","OTIR","OTDR"]
# I/O
out += ["IN A,($FE)","OUT ($FE),A","IN A,(C)","OUT (C),A"]
for s in regs8: out += [f"IN {s},(C)", f"OUT (C),{s}"]
# I/R registers
out += ["LD A,I","LD A,R","LD I,A","LD R,A"]
# IX/IY family
for ix in ["IX","IY"]:
    out += [f"LD {ix},$1234", f"PUSH {ix}", f"POP {ix}", f"ADD {ix},BC", f"ADD {ix},DE", f"ADD {ix},SP",
            f"INC {ix}", f"DEC {ix}", f"LD SP,{ix}", f"JP ({ix})"]
    for s in regs8:
        out += [f"LD {s},({ix}+2)", f"LD ({ix}+2),{s}"]
    out += [f"LD ({ix}+2),5", f"ADD A,({ix}+2)", f"INC ({ix}+2)", f"DEC ({ix}+2)", f"CP ({ix}+2)"]
# Undocumented IX/IY half registers (IXH/IXL/IYH/IYL). The completeness checker
# skips any line the reference assembler rejects, so only the legal forms below
# are compared; illegal mixes (LD IXH,IYL, LD (IX+d),IXH) are covered by the
# dedicated half-register test, not here.
for ix,(h,l) in [("IX",("IXH","IXL")),("IY",("IYH","IYL"))]:
    for hr in (h,l):
        out.append(f"LD {hr},A")
        out.append(f"LD A,{hr}")
        out.append(f"LD {hr},5")
        out.append(f"INC {hr}")
        out.append(f"DEC {hr}")
        out.append(f"ADD A,{hr}")
        out.append(f"SUB {hr}")
        out.append(f"AND {hr}")
        out.append(f"OR {hr}")
        out.append(f"XOR {hr}")
        out.append(f"CP {hr}")
    out.append(f"LD {h},{l}")
    out.append(f"LD {l},{h}")

print("\n".join(out))
