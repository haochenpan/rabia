Require Import ssreflect.
From Coq Require Export Morphisms RelationClasses List Bool Utf8 Setoid Lia.
Set Default Proof Using "Type".
Global Set Bullet Behavior "Strict Subproofs".
Global Open Scope general_if_scope.

Section definitions.
  Context {A: Type}. Context `(R : relation A).
  Inductive rtc : relation A :=
    | rtc_refl x : rtc x x
    | rtc_l x y z : R x y → rtc y z → rtc x z.
End definitions.

Section weak_mvc.

Parameter state : Set.
Parameter node : Set.
Definition phase := nat.
Parameter proposal_value : Set.
Inductive state_value := | v0 | v1 | vquestion.
Parameter set_majority : Set.
Parameter set_f_plus_1 : Set.

Parameter member_maj : state → node → set_majority → Prop.
Parameter member_fp1 : state → node → set_f_plus_1 → Prop.
Parameter in_phase : state → node → phase → Prop.
Parameter propose : state → node → proposal_value → Prop.
Parameter vote_rnd1 : state → node → phase → state_value → Prop.
Parameter vote_rnd2 : state → node → phase → state_value → Prop.
Parameter decision_bc : state → node → phase → state_value → Prop.
Parameter decision_full_val : state → node → phase → proposal_value → Prop.
Parameter decision_full_noval : state → node → phase → Prop.
Parameter coin : state → phase → state_value → Prop.

(* In the Ivy model, succ is defined as a relation, however now that we're
   instantiating the phases to be concrete numbers, we will define it to be the successor function *)
Definition succ := S.

Parameter steps : state → state → Prop.
Parameter initial : state → Prop.

(* A predicate is in invariant if it holds in all states σ' reachable in 0 or
   more steps from an initial state σ. *)

Definition invariant (pred: state → Prop) :=
  (∀ σ σ', initial σ → rtc steps σ σ' → pred σ').

Implicit Types σ : state.
Implicit Types N : node.
Implicit Types P : phase.
Implicit Types ϕ : state → Prop.

Definition state_value_locked σ P V :=
  ∀ N Valt, vote_rnd1 σ N P Valt → Valt = V.

Definition started σ P := ∃ N V, vote_rnd1 σ N P V.

Definition good σ P :=
  started σ P ∧
  (∀ P0, lt P0 P → started σ P0) ∧
  (∀ P0 V0, lt P0 P ∧
            started σ P ∧
            ((∃ N, decision_bc σ N P0 V0) ∨ state_value_locked σ P0 V0)
            → state_value_locked σ P V0).

(* Specifications from protocol isolate *)
Definition vote_rnd1_pred_rnd1 σ :=
  ∀ N1 P V1, vote_rnd1 σ N1 (succ P) V1 → ∃ N2, vote_rnd1 σ N2 P V1.
Axiom vote_rnd1_pred_rnd1_invariant : invariant vote_rnd1_pred_rnd1.

Definition vl_decision_bc_agree σ :=
  ∀ P N2 V2, ∀ V, state_value_locked σ P V ∧ decision_bc σ N2 P V2 → V = V2.
Axiom vl_decision_bc_agree_invariant : invariant vl_decision_bc_agree.

Definition decision_bc_same_round_agree σ :=
  ∀ P N1 V1 N2 V2,
    decision_bc σ N1 P V1 ∧ decision_bc σ N2 P V2 → V1 = V2.
Axiom decision_bc_same_round_agree_invariant : invariant decision_bc_same_round_agree.

Definition good_succ_good σ :=
  (∀ P, good σ P ∧ started σ (succ P) → good σ (succ P)).
Axiom good_succ_good_invariant : invariant good_succ_good.

Definition decision_full_val_agree σ :=
  ∀ N1 P1 V1 N2 P2 V2,
    decision_full_val σ N1 P1 V1 ->
    decision_full_val σ N2 P2 V2 ->
    V1 = V2.
Axiom decision_full_val_agree_invariant : invariant decision_full_val_agree.

Definition decision_full_val_inv σ :=
  ∀ N P V,
    decision_full_val σ N P V->
    decision_bc σ N P v1.
Axiom decision_full_val_inv_invariant : invariant decision_full_val_inv.

Definition decision_full_val_validity σ :=
  ∀ N P V,
    decision_full_val σ N P V->
    ∃ N, propose σ N V.
Axiom decision_full_val_validity_invariant : invariant decision_full_val_validity.

Definition decision_full_noval_inv σ :=
  ∀ N P,
    decision_full_noval σ N P ->
    decision_bc σ N P v0.
Axiom decision_full_noval_inv_invariant : invariant decision_full_noval_inv.

(* Specification from wrapper2 isolate *)
Definition good_zero σ :=
  (started σ 0 → good σ 0).
Axiom good_zero_invariant : invariant good_zero.

(* Specification from wrapper3 isolate *)
Definition started_pred σ :=
  (∀ P, started σ (succ P) → started σ P).
Axiom started_pred_invariant : invariant started_pred.

(* Specification from wrapper4 isolate *)
Definition decision_bc_started σ :=
  ∀ N P V2, decision_bc σ N P V2 → started σ P.
Axiom decision_bc_started_invariant : invariant decision_bc_started.

(* Specification from wrapper5 isolate *)
Definition decision_bc_vote_rnd1 σ :=
  ∀ N P V, decision_bc σ N P V → ∃ N2, vote_rnd1 σ N2 P V.
Axiom decision_bc_vote_rnd1_invariant : invariant decision_bc_vote_rnd1.

Definition started_good σ :=
  ∀ P, started σ P → good σ P.
Lemma started_good_invariant : invariant started_good.
Proof.
  intros σ σ' Hinit Hreach.
  assert (Hgsg: good_succ_good σ') by eauto using good_succ_good_invariant.
  assert (Hgz0: good_zero σ') by eauto using good_zero_invariant.
  assert (Hhp: started_pred σ') by eauto using  started_pred_invariant.
  clear Hinit Hreach σ.

  intros P Hstarted. induction P.
  - eauto.
  - eapply Hgsg; split; auto.
Qed.

Lemma invariant_weakening ϕ1 ϕ2:
  invariant ϕ1 →
  (∀ σ, ϕ1 σ → ϕ2 σ) →
  invariant ϕ2.
Proof. firstorder. Qed.

Definition validity_bc σ :=
  ∀ N1 P1 V1, decision_bc σ N1 P1 V1 → ∃ N2, vote_rnd1 σ N2 0 V1.

Lemma validity_bc_invariant :
  invariant validity_bc.
Proof.
  intros σ σ' Hinit Hreach.
  assert (Hdr1: decision_bc_vote_rnd1 σ') by eauto using decision_bc_vote_rnd1_invariant.
  assert (Hvr1_pred_r1: vote_rnd1_pred_rnd1 σ') by eauto using vote_rnd1_pred_rnd1_invariant.
  clear Hinit Hreach σ.

  (* Strengthen the induction hypothesis *)
  cut (∀ N1 P1 V1, vote_rnd1 σ' N1 P1 V1 → ∃ N2, vote_rnd1 σ' N2 0 V1).
  { firstorder (eauto using Hdr1). }
  intros N1 P1. revert N1.
  induction P1 => N1 V1 Hdec.
  - eauto.
  - eapply Hvr1_pred_r1 in Hdec as (?&?). eapply IHP1; eauto.
Qed.

Definition agreement_bc σ :=
  ∀ N1 P1 V1 N2 P2 V2,
      decision_bc σ N1 P1 V1 →
      decision_bc σ N2 P2 V2 →
      V1 = V2.

Lemma agreement_bc_invariant :
  invariant agreement_bc.
Proof.
  intros σ σ' Hinit Hreach.
  assert (Hstarted: started_good σ') by eauto using started_good_invariant.
  assert (Hvld_agree: vl_decision_bc_agree σ') by eauto using vl_decision_bc_agree_invariant.
  assert (Hdsr_agree: decision_bc_same_round_agree σ') by eauto using decision_bc_same_round_agree_invariant.
  assert (Hdstarted: decision_bc_started σ') by eauto using decision_bc_started_invariant.
  clear Hinit Hreach σ.

  (* WLOG assume P1 <= P2 *)
  cut (∀ N1 P1 V1 N2 P2 V2,
      P1 ≤ P2 →
      decision_bc σ' N1 P1 V1 →
      decision_bc σ' N2 P2 V2 →
      V1 = V2).
  { intros Hlem N1 P1 V1 N2 P2 V2 Hdec1 Hdec2.
    assert (P1 ≤ P2 ∨ P2 ≤ P1) as [Hle|Hle] by lia.
    - eapply Hlem; (try eapply Hle); eauto.
    - symmetry. eapply Hlem; (try eapply Hle); eauto.
  }

  intros N1 P1 V1 N2 P2 V2 Hle Hdec1 Hdec2.
  destruct Hle as [|P2 ].
  - eapply Hdsr_agree; eauto.
  - assert (state_value_locked σ' (S P2) V1).
    { eapply Hstarted; eauto.
      split; [| split]; last first.
      { left. eexists; eauto. }
      { eauto. }
      lia.
    }
    eapply Hvld_agree; eauto.
Qed.

(* This part is proved in Ivy already *)
Definition agreement1 σ :=
  ∀ N1 P1 V1 N2 P2 V2,
      decision_full_val σ N1 P1 V1 →
      decision_full_val σ N2 P2 V2 →
      V1 = V2.
Lemma agreement1_invariant :
  invariant agreement1.
Proof. apply decision_full_val_agree_invariant. Qed.

(* This is derived from agreement of BC *)
Definition agreement2 σ :=
  ∀ N1 P1 V1 N2 P2,
      decision_full_val σ N1 P1 V1 →
      decision_full_noval σ N2 P2 →
      False.
Lemma agreement2_invariant :
  invariant agreement2.
Proof.
  intros σ1 σ2 Hinit Hreach N1 P1 V1 N2 P2 Hdec1 Hdec2.
  eapply decision_full_val_inv_invariant in Hdec1; eauto.
  eapply decision_full_noval_inv_invariant in Hdec2; eauto.
  assert (v0 = v1).
  { eapply agreement_bc_invariant; try eassumption. }
  congruence.
Qed.

(* This part is proved in Ivy already *)
Definition validity σ :=
  ∀ N P V,
    decision_full_val σ N P V->
    ∃ N, propose σ N V.
Lemma validity_invariant :
  invariant validity.
Proof. apply decision_full_val_validity_invariant. Qed.

End weak_mvc.
