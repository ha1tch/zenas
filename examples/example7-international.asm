; Example 7: international character set support
.ORG $8000

START:
        ; standard ASCII
        DEFB "Hello World", 13, 0
        
        ; Portuguese characters (TK90X)
        DEFB "São Paulo", 0
        DEFB "Coração", 0
        
        ; Spanish characters
        DEFB "Año Nuevo", 0
        DEFB "¿Cómo está?", 0
        
        ; mixed formats
        DEFB "Price: ", &A3, " 5.00", 0    ; £ symbol + text
        
        ; traditional formats still work
        DEFB $FF, %11110000, 123
        
        HALT

.END START