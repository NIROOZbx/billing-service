-- name: GetPlanByID :one
SELECT *
FROM public.plans
WHERE id = $1
LIMIT 1;

-- name: GetPlanByName :one
SELECT * FROM public.plans 
WHERE name = $1 LIMIT 1;
