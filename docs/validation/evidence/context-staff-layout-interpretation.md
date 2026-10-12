# Staff layout slots and visible publishers

The pinned MusicXML 4.0 schema distinguishes two conditions:

- `staff-layout` measures from the previous staff in the system and ignores
  staff-distance for the first staff in that system (musicxml.xsd 4479–4486).
- A print-local `system-layout` need only be read from the first visible part
  (musicxml.xsd 4511).

They are not the same predicate. `staff-details` explicitly describes cutaway
spacing when print-object is no and print-spacing is yes. A proved retained
vertical slot therefore cannot be erased just because it is not visible. A
later visible part may supply system-layout while its staff still has a
preceding retained layout slot. Known print-spacing=no can remove that slot;
unknown visibility or spacing leaves only the affected relationship unknown.

The implementation maintains separate first-visible-publisher and occupied-slot
proofs. It does not expand enormous staff counts into arrays. Part/staff identity,
encoded system boundaries, conditional scope and source/model availability remain
independent. This is not a renderer's general empty-staff or cutaway policy.

Primary sources:

- https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/staff-layout/
- https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/system-layout/
- https://www.w3.org/2021/06/musicxml40/musicxml-reference/elements/staff-details/
