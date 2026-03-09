# CRUD Test Fixtures — Employee Management System

This directory contains YAML fixture files for CRUD integration tests. All models belong to a single **Employee Management System** domain with realistic business relationships.

## ER Relationships

```
Operator (test_operator)              ← System operator (audit user lookup)
├── referenced by Employee.created_by / updated_by
├── referenced by Department.created_by / updated_by

Employee (test_employee)              ← Main CRUD entity
├── created_by / updated_by → Operator.id
├── department_id → Department.id    (logical, not FK)

Department (test_department)          ← Tree structure (org chart)
├── self-ref → parent_id
├── 1:N → Employee via department_id

ProjectAssignment (test_project_assignment) ← Composite PK
├── project_code + employee_id (composite PK)
├── employee_id → Employee.id       (logical)

ExportEmployee (export_employee)      ← Pure export data struct
ImportEmployee (import_employee)      ← Pure import data struct
```

## Data Summary

| Model | Table | Records | Base Model | Key Traits |
|-------|-------|---------|------------|------------|
| Operator | test_operator | 8 | `orm.IDModel` | id + name only, audit lookup |
| Employee | test_employee | 25 | `orm.Model` | Full audit, status: active(17)/inactive(5)/on_leave(3), ages 22–50 unique |
| Department | test_department | 18 | `orm.Model` | Tree: 4 roots, max depth 3, hierarchical codes |
| ProjectAssignment | test_project_assignment | 15 | None | Composite PK, 4 projects, emp001 in 2 projects |
| ExportEmployee | export_employee | 10 | None | Pure data struct with tabular tags |
| ImportEmployee | import_employee | 1 | None | Pure data struct with tabular + validate tags |

## Employee Distribution

- **Status**: active=17, inactive=5, on_leave=3
- **Age range**: 22–50 (all unique)
  - `age < 30` → 8 employees
  - `30 ≤ age ≤ 40` → 11 employees
  - `age > 40` → 6 employees
  - `Between(25, 35)` → 11 employees
- **Position**: Senior Engineer(5), Engineer(4), Designer(3), Product Manager(3), Analyst(3), Intern(3), Team Lead(2), Director(2)
- **Description**: 20 non-empty, 5 empty (emp005, emp010, emp015, emp020, emp025)

## Department Tree

```
Engineering (dept001, ENG, sort=1)
├── Backend (dept005, ENG-BE, sort=1)
│   ├── API Team (dept010, ENG-BE-API, sort=1)
│   └── Data Team (dept011, ENG-BE-DATA, sort=2)
├── Frontend (dept006, ENG-FE, sort=2)
│   └── Mobile Team (dept012, ENG-FE-MOB, sort=1)
├── DevOps (dept013, ENG-DEVOPS, sort=3)
└── QA (dept014, ENG-QA, sort=4)

Product (dept002, PRD, sort=2)
├── Design (dept007, PRD-DSN, sort=1)
└── Research (dept015, PRD-RES, sort=2)

Marketing (dept003, MKT, sort=3)
├── Content (dept008, MKT-CNT, sort=1)
├── Growth (dept016, MKT-GRW, sort=2)
└── Brand (dept017, MKT-BRD, sort=3)

HR (dept004, HR, sort=4)
├── Recruitment (dept009, HR-REC, sort=1)
└── Training (dept018, HR-TRN, sort=2)
```
