; Test MSX Japanese Character Support
.ORG $8000

START:
        ; Test Japanese Katakana
        DEFB "ゲームオーバー", 0     ; "Game Over" in Katakana
        DEFB "スタート", 0         ; "Start" in Katakana
        
        ; Mixed Japanese/English
        DEFB "MSXゲーム", 0
        
        ; Traditional numbers and symbols
        DEFB "1UP", 0
        DEFB $FF, $00
        
        HALT

.END START