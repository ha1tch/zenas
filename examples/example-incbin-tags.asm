; example-incbin-tags.asm - INCBIN and composable build tags
;
; Select build variants with tags (Go-build-tag style):
;   zenas assemble example-incbin-tags.asm out.bin --tag release
;   zenas assemble example-incbin-tags.asm out.bin --tag debug --tag plus3
;
; Each --tag NAME defines:
;   ZENAS_TAG_NAME    = 1            presence flag (for IFDEF / IF / AND / OR / NOT)
;   ZENAS_TAGBIT_NAME = 1 << bit     this tag's bit (bits follow command-line order)
; and ZENAS_TAGS is the OR of all set tags' bits - handy to embed as a build stamp.
;
; IF conditions support AND, OR, NOT and parentheses, so tags compose freely.

                ORG     $8000

start:
                ; Presence test - the simplest form.
                IFDEF   ZENAS_TAG_debug
                DB      $DE, $AD                ; debug-only marker
                ENDIF

                ; Boolean composition of tags.
                IF      ZENAS_TAG_debug AND ZENAS_TAG_plus3
                DB      $D3                      ; debug build for the +3
                ENDIF

                IF      ZENAS_TAG_release OR NOT ZENAS_TAG_debug
                DB      $5E                      ; shipping path
                ENDIF

                ; Embed the combined tag bitmask as a one-byte build stamp.
build_stamp:    DB      ZENAS_TAGS

                ; Embed a raw binary asset; optional skip/length select a slice.
sprite:         INCBIN  "asset.bin"
sprite_head:    INCBIN  "asset.bin",0,4

end:            DB      $FF
